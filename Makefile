.PHONY: all build-web build-go build clean dev run help

# Default target
all: build

# Build everything
build: build-web build-go

# Build the Vite web application
build-web:
	@echo "Building Vite web application..."
	cd web && npm install && npm run build

# Build the Go binary
build-go:
	@echo "Building Go binary..."
	go build -o bin/server main.go

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf web/dist
	rm -rf web/node_modules
	rm -rf bin
	rm -rf /tmp/badger

# Run Vite dev server
dev:
	@echo "Starting Vite dev server..."
	cd web && npm run dev

# Run the Go server
run: build
	@echo "Starting server..."
	./bin/server

# Install dependencies
install:
	@echo "Installing dependencies..."
	cd web && npm install
	go mod download

# Run the server in development mode (rebuild on changes)
watch:
	@echo "Starting server with auto-reload..."
	go run main.go

# Help target
help:
	@echo "Available targets:"
	@echo "  make build      - Build web and Go binary"
	@echo "  make build-web  - Build only Vite web app"
	@echo "  make build-go   - Build only Go binary"
	@echo "  make clean      - Remove build artifacts"
	@echo "  make dev        - Run Vite dev server"
	@echo "  make run        - Build and run Go server"
	@echo "  make install    - Install dependencies"
	@echo "  make watch      - Run Go server with auto-reload"
	@echo "  make help       - Show this help message"