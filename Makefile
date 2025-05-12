# Get version info for the build
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date +%FT%T%z)

# Run lint checks:
lint:
	@echo "Linting application source code..."
	go fmt ./...
	go vet ./...
	@echo "Linting complete"

# Build the application with version info
build: lint
	@echo "Building SheetBridge binary..."
	mkdir -p bin
	go build -ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" -o bin/SheetBridge cmd/server/main.go
	@echo "Binary built at bin/SheetBridge"

# Run the application after building
run: build
	./bin/SheetBridge

# Clean build artifacts
clean:
	rm -rf bin/

# Test everything
test:
	go test -v ./...

# Setup test database
setup-test-db:
	@echo "Creating test database..."
	psql -U postgres -c "DROP DATABASE IF EXISTS spreadsheet_db_test;"
	psql -U postgres -c "CREATE DATABASE spreadsheet_db_test;"

.PHONY: build run clean test lint
