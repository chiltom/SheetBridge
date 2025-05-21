.PHONY: all run build tidy clean test help css-build css-watch check-tailwind-cli

# Variables
APP_NAME := sheetbridge
BIN_DIR := ./bin
CMD_DIR := ./cmd/server
BUILD_OUTPUT := $(BIN_DIR)/$(APP_NAME)

# Go commands
GO := go
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean
GOTEST := $(GO) test
GOMODTIDY := $(GO) mod tidy
GOMODVENDOR := $(GO) mod vendor

# Tailwind CLI command
TAILWIND_CLI := ./tailwindcss

# CSS files
CSS_INPUT_FILE := ./web/static/css/input.css
CSS_OUTPUT_FILE := ./web/static/css/output.css
CONFIG_FILE := ./tailwind.config.js

all: build

# Check if Tailwind CLI is available
check-tailwind-cli:
	@if ! command -v $(TAILWIND_CLI) &> /dev/null && ! [ -f "$(TAILWIND_CLI)" ]; then \
		echo "Warning! Tailwind CSS CLI ('tailwindcss') not found at $(TAILWIND_CLI) or on PATH."; \
		echo "CSS will not be built or watched. Ensure 'web/static/css/output.css' is up-to-date or not needed for this operation."; \
	fi

# Build CSS only if Tailwind CLI is found
css-build: check-tailwind-cli
	@if command -v $(TAILWIND_CLI) &> /dev/null || [ -f "$(TAILWIND_CLI)" ]; then \
		echo "Building CSS with Tailwind CLI..."; \
		$(TAILWIND_CLI) -i $(CSS_INPUT_FILE) -o $(CSS_OUTPUT_FILE) --minify; \
	else \
		echo "Skipping CSS build: Tailwind CLI not found."; \
	fi

# Watch CSS for changes only if Tailwind CLI is found
css-watch: check-tailwind-cli
	@if command -v $(TAILWIND_CLI) &> /dev/null || [ -f "$(TAILWIND_CLI)" ]; then \
		echo "Watching CSS changes with Tailwind CLI..."; \
		$(TAILWIND_CLI) -i $(CSS_INPUT_FILE) -o $(CSS_OUTPUT_FILE) --watch; \
	else \
		echo "Cannot watch CSS: Tailwind CLI not found. Please install it for live CSS development."; \
		exit 1; # Exit because css-watch is explicitly for CSS dev
	fi

# Build Go application binary
go-build: tidy vendor
	@echo "Building Go application $(APP_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BUILD_OUTPUT) $(CMD_DIR)
	@echo "Go build complete: $(BUILD_OUTPUT)"

# Run the application using Air for live reloading
# Tries to build CSS once. If Tailwind CLI is not found, it skips CSS build.
run: css-build go-build # Build CSS then Go binary for Air to use
	@echo "Starting application with Air live reload..."
	@echo "If Tailwind CLI is available, run 'make css-watch' in a separate terminal for live CSS rebuilds."
	air

# Build the whole application
build: css-build go-build
	@echo "Full build process complete."
	@echo "Output: $(BUILD_OUTPUT)"
	@echo "CSS: $(CSS_OUTPUT_FILE) (attempted build)"

# Tidy Go modules
tidy:
	@echo "Tidying Go modules..."
	$(GOMODTIDY)

# Vendor dependencies
vendor:
	@echo "Vendoring dependencies..."
	$(GOMODVENDOR)

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BUILD_OUTPUT) $(APP_NAME)
	@rm -rf $(BIN_DIR) ./tmp
	@rm -f $(CSS_OUTPUT_FILE)
	@echo "Clean complete."

# Test (unit testing incoming)
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	# go tool cover -html=cover.out # To view coverage

# For deployment (example, adjust as needed)
# This is a manual step, not typically part of `make run` for dev
deploy-prep: build
	@echo "Preparing for production deployment..."
	@echo "Ensure $(BUILD_OUTPUT) is copied to /opt/$(APP_NAME)/$(APP_NAME)"
	@echo "Ensure .env file or environment variables are set on the target server."
	@echo "Ensure 'migrations' directory is available if you run migrations on deployment."
	@echo "'web/static' and 'web/templates' are not strictly needed if embedded, but good for reference."

help:
	@echo "Available commands for $(APP_NAME):"
	@echo "  run                : Run Go app with Air (tries to build CSS once, then Go binary)"
	@echo "  build              : Build Go app and try to build CSS (for production/deployment)"
	@echo "  go-build           : Build only the Go application binary"
	@echo "  tidy               : Tidy Go modules"
	@echo "  vendor             : Vendor Go dependencies"
	@echo "  clean              : Clean build artifacts"
	@echo "  test               : Run Go tests"
	@echo "  css-build          : Try to build CSS once (requires Tailwind CLI)"
	@echo "  css-watch          : Watch and rebuild CSS automatically (requires Tailwind CLI)"
	@echo "  check-tailwind-cli : Checks for Tailwind CLI and prints status"
