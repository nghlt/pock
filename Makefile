.PHONY: build install clean

BINARY := pock
INSTALL_DIR ?= /usr/local/bin

build:
	@echo "🔨 Building $(BINARY)..."
	@if go build -o $(BINARY) .; then \
		echo "✅ Build succeeded: ./$(BINARY)"; \
		./$(BINARY) version; \
	else \
		echo "❌ Build failed"; \
		exit 1; \
	fi

install:
	@echo "🔨 Installing $(BINARY) to $(INSTALL_DIR)..."
	@if go build -o $(INSTALL_DIR)/$(BINARY) .; then \
		echo "✅ Installed: $(INSTALL_DIR)/$(BINARY)"; \
		$(INSTALL_DIR)/$(BINARY) version; \
	else \
		echo "❌ Install failed"; \
		exit 1; \
	fi

clean:
	@rm -f $(BINARY)
	@echo "🧹 Removed ./$(BINARY)"
