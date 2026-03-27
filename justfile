# Default recipe
default: check

# Build the zigbee-skill binary
build:
    mkdir -p bin
    go build -o bin/zigbee-skill ./cmd/cli

# Clean build artifacts
clean:
    rm -rf bin/

# Run tests
test:
    go test ./...

# Lint: format check, golangci-lint, go vet
lint:
    gofmt -l .
    golangci-lint run ./...
    go vet ./...

# Quality gate: lint + test
check: lint test
