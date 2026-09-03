package session

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

// Message types
const (
	MsgInput  byte = 1
	MsgOutput byte = 2
	MsgResize byte = 3
	MsgExit   byte = 4
)

// ringBuffer is a fixed-size circular buffer for session output history
type ringBuffer struct {
	buf  []byte
	size int
	head int
	full bool
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{
		buf:  make([]byte, size),
		size: size,
	}
}

func (r *ringBuffer) Write(p []byte) {
	if len(p) == 0 {
		return
	}
	if len(p) >= r.size {
		copy(r.buf, p[len(p)-r.size:])
		r.head = 0
		r.full = true
		return
	}
	n := len(p)
	end := r.size - r.head
	if n <= end {
		copy(r.buf[r.head:], p)
		r.head += n
		if r.head == r.size {
			r.head = 0
			r.full = true
		}
	} else {
		copy(r.buf[r.head:], p[:end])
		copy(r.buf[0:], p[end:])
		r.head = n - end
		r.full = true
	}
}

func (r *ringBuffer) Bytes() []byte {
	if !r.full {
		out := make([]byte, r.head)
		copy(out, r.buf[:r.head])
		return out
	}
	out := make([]byte, r.size)
	copy(out, r.buf[r.head:])
	copy(out[r.size-r.head:], r.buf[:r.head])
	return out
}

// clientInfo holds per-client state
type clientInfo struct {
	rows      uint16
	cols      uint16
	send      chan []byte
	closeOnce sync.Once
}

func (ci *clientInfo) close() {
	ci.closeOnce.Do(func() {
		close(ci.send)
	})
}

// Server manages a session
type Server struct {
	session     *Session
	pty         *PTY
	listener    net.Listener
	clients     map[net.Conn]*clientInfo
	mu          sync.RWMutex
	done        chan struct{}
	ptyExited   bool
	outputBuf   *ringBuffer
	outputBufMu sync.Mutex
	hadClient   bool
	currentRows uint16
	currentCols uint16
}

// NewServer creates a new server for a session
func NewServer(name string, command []string) (*Server, error) {
	// Ensure data directory exists
	if _, err := EnsureDataDir(); err != nil {
		return nil, err
	}

	// Check if session already exists
	if Exists(name) {
		return nil, os.ErrExist
	}

	// Start PTY
	p, err := StartPTY(name, command)
	if err != nil {
		return nil, err
	}

	// Create Unix socket
	sockPath, err := SocketPath(name)
	if err != nil {
		_ = p.Close()
		return nil, err
	}

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		_ = p.Close()
		return nil, err
	}

	// Save session info
	sess := &Session{
		Name:       name,
		PID:        os.Getpid(),
		Command:    command,
		LastActive: time.Now(),
	}
	if err := sess.Save(); err != nil {
		_ = listener.Close()
		_ = p.Close()
		return nil, err
	}

	return &Server{
		session:   sess,
		pty:       p,
		listener:  listener,
		clients:   make(map[net.Conn]*clientInfo),
		done:      make(chan struct{}),
		outputBuf: newRingBuffer(1024 * 1024), // 1MB circular buffer
	}, nil
}

// resizePTY resizes the PTY only when dimensions have actually changed
func (s *Server) resizePTY(rows, cols uint16) {
	if rows == 0 || cols == 0 {
		return
	}
	if rows == s.currentRows && cols == s.currentCols {
		return
	}
	s.currentRows = rows
	s.currentCols = cols
	_ = s.pty.Resize(rows, cols)
}

// Run starts the server
func (s *Server) Run() error {
	// Handle PTY output in background
	go s.handlePTYOutput()

	// Wait for PTY process to exit
	go func() {
		_ = s.pty.Wait()
		s.mu.Lock()
		s.ptyExited = true
		s.mu.Unlock()

		// Notify all clients that PTY exited
		s.broadcast(MsgExit, nil)

		// Wait briefly for client to connect if none yet
		for range 50 { // 5 seconds max
			s.mu.RLock()
			hadClient := s.hadClient
			s.mu.RUnlock()
			if hadClient {
				break
			}
			sleepMs(100)
		}

		// Small delay to ensure exit message is sent
		sleepMs(100)
		s.Shutdown()
	}()

	// Accept connections
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return nil
			default:
				continue
			}
		}
		go s.handleClient(conn)
	}
}

// Shutdown stops the server
func (s *Server) Shutdown() {
	select {
	case <-s.done:
		return
	default:
		close(s.done)
	}

	_ = s.listener.Close()
	_ = s.pty.Close()

	s.mu.Lock()
	for conn, info := range s.clients {
		if info != nil {
			info.close()
		}
		_ = conn.Close()
	}
	s.mu.Unlock()

	// Clean up session files
	_ = Remove(s.session.Name)
}

// handlePTYOutput reads from PTY and broadcasts to all clients
func (s *Server) handlePTYOutput() {
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-s.done:
			return
		default:
		}

		n, err := s.pty.File.Read(buf)
		if err != nil {
			return
		}
		if n > 0 {
			// Store in ring buffer for late-connecting clients (zero allocations)
			s.outputBufMu.Lock()
			s.outputBuf.Write(buf[:n])
			s.outputBufMu.Unlock()

			s.broadcast(MsgOutput, buf[:n])
		}
	}
}

// handleClient handles a single client connection
func (s *Server) handleClient(conn net.Conn) {
	sendChan := make(chan []byte, 64)
	info := &clientInfo{send: sendChan}

	s.mu.Lock()
	s.clients[conn] = info
	s.hadClient = true
	// Update last active time
	s.session.LastActive = time.Now()
	_ = s.session.Save()
	s.mu.Unlock()

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		for data := range sendChan {
			if err := writeMessage(conn, MsgOutput, data); err != nil {
				_ = conn.Close()
				return
			}
		}
	}()

	// Send buffered output to new client
	s.outputBufMu.Lock()
	buffered := s.outputBuf.Bytes()
	s.outputBufMu.Unlock()
	if len(buffered) > 0 {
		_ = writeMessage(conn, MsgOutput, buffered)
	}

	// If PTY already exited, send exit message and close
	s.mu.RLock()
	ptyExited := s.ptyExited
	s.mu.RUnlock()
	if ptyExited {
		_ = writeMessage(conn, MsgExit, nil)
		s.mu.Lock()
		delete(s.clients, conn)
		info.close()
		s.mu.Unlock()
		_ = conn.Close()
		<-clientDone
		return
	}

	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		info.close()
		// Resize PTY to a remaining client's size if any
		for _, remaining := range s.clients {
			if remaining != nil && remaining.rows > 0 && remaining.cols > 0 {
				s.resizePTY(remaining.rows, remaining.cols)
				break
			}
		}
		s.mu.Unlock()
		_ = conn.Close()
		<-clientDone
	}()

	// Read messages from client
	for {
		select {
		case <-s.done:
			return
		default:
		}

		msgType, data, err := readMessage(conn)
		if err != nil {
			return
		}

		switch msgType {
		case MsgInput:
			// Resize PTY only if active client's size changed
			s.mu.Lock()
			if curInfo := s.clients[conn]; curInfo != nil && curInfo.rows > 0 && curInfo.cols > 0 {
				s.resizePTY(curInfo.rows, curInfo.cols)
			}
			s.mu.Unlock()
			_, _ = s.pty.File.Write(data)
		case MsgResize:
			if len(data) >= 4 {
				rows := binary.BigEndian.Uint16(data[0:2])
				cols := binary.BigEndian.Uint16(data[2:4])
				s.mu.Lock()
				if curInfo := s.clients[conn]; curInfo != nil {
					curInfo.rows = rows
					curInfo.cols = cols
				}
				s.resizePTY(rows, cols)
				s.mu.Unlock()
			}
		}
	}
}

// broadcast sends a message to all connected clients without blocking PTY
func (s *Server) broadcast(msgType byte, data []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for conn, info := range s.clients {
		if info != nil && info.send != nil && msgType == MsgOutput {
			pkt := make([]byte, len(data))
			copy(pkt, data)
			select {
			case info.send <- pkt:
			default:
				// Client write buffer is full; drop frame to prevent freezing server
			}
		} else {
			_ = writeMessage(conn, msgType, data)
		}
	}
}

// Protocol helpers

// Message format: [type:1byte][length:4bytes][data:N bytes]

func writeMessage(w io.Writer, msgType byte, data []byte) error {
	var header [5]byte
	header[0] = msgType
	binary.BigEndian.PutUint32(header[1:], uint32(len(data)))

	if len(data) == 0 {
		_, err := w.Write(header[:])
		return err
	}

	// Use writev on socket connections to send header + data in a single syscall
	if _, ok := w.(net.Conn); ok {
		bufs := net.Buffers{header[:], data}
		_, err := bufs.WriteTo(w)
		return err
	}

	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func readMessage(r io.Reader) (byte, []byte, error) {
	var header [5]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}
	msgType := header[0]
	length := binary.BigEndian.Uint32(header[1:])
	if length > 1024*1024 { // 1MB max
		return 0, nil, io.ErrShortBuffer
	}
	if length == 0 {
		return msgType, nil, nil
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return 0, nil, err
	}
	return msgType, data, nil
}

func sleepMs(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
