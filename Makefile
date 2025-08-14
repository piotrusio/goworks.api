# Go settings
GO_FILES := $(shell find . -type f -name '*.go' -not -path "./vendor/*")
APP_NAME := api
CMD_PATH := ./cmd/api

# Build binary
build:
	go build -o bin/$(APP_NAME) $(CMD_PATH)

# Run the app (dev only)
run:
	go run $(CMD_PATH)

# Run tests
test:
	go test ./... -cover

# Format code
fmt:
	goimports -w .

# Lint (requires golangci-lint or install one)
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -rf bin

# Help
help:
	@echo "Usage: make [target]"
	@echo "Targets:"
	@echo "  build     - Compile the app to ./bin"
	@echo "  run       - Run the app (go run)"
	@echo "  test      - Run tests with coverage"
	@echo "  fmt       - Format all Go files"
	@echo "  lint      - Run linter"
	@echo "  clean     - Remove build artifacts" 