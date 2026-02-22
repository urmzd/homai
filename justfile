# Default recipe
default: check

# Build all binaries
build:
    #!/usr/bin/env bash
    for main_file in $(find cmd -type f -name main.go 2>/dev/null); do
        bin_name=$(dirname "$main_file" | sed 's|cmd/|bin/|')
        echo "Building $bin_name"
        mkdir -p "$(dirname "$bin_name")"
        go build -o "$bin_name" "$main_file"
    done

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

# Generate swagger documentation
swagger:
    swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal

# Build and run API with live reload
run:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Starting API server with live reload..."
    air

# Start the API server only
start-api:
    air

# Open database in sqlite3
open-db:
    sqlite3 ~/.config/homai/homai.db

# Reset database (delete file)
reset-db:
    rm -f ~/.config/homai/homai.db
