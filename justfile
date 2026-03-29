# Default recipe
default: check

# Build the zigbee-skill binary to bin/
build:
    mkdir -p bin
    go build -o bin/zigbee-skill ./cmd/zigbee-skill

# Install zigbee-skill to $GOPATH/bin (or $HOME/go/bin)
install:
    go install ./cmd/zigbee-skill

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

# Format code
fmt:
    gofmt -w .

# Tidy modules
tidy:
    go mod tidy

# Run govulncheck
vuln:
    govulncheck ./...

# Record showcase with teasr
record:
    teasr showme

# Quality gate: lint + test
check: lint test

# Full CI gate: lint + build + test
ci: lint build test
