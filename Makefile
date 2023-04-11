# Set the default shell
SHELL := /bin/bash

# Set the default target
.DEFAULT_GOAL := build

# Find all main.go files in the cmd directory
MAIN_GO_FILES := $(shell find cmd -type f -name main.go)

# Define the binary output files based on the main.go files
BINARIES_x86_64 := $(patsubst cmd/%/main.go,bin/x86_64/%,$(MAIN_GO_FILES))
BINARIES_ARM64 := $(patsubst cmd/%/main.go,bin/arm64/%,$(MAIN_GO_FILES))

# Build all binaries
build: build-x86_64 build-arm64

# Build x86_64 binaries
build-x86_64: $(BINARIES_x86_64)

# Build ARM64 binaries
build-arm64: $(BINARIES_ARM64)

# Rule to build each x86_64 binary
bin/x86_64/%: cmd/%/main.go
	@echo "Building $@"
	@mkdir -p $(dir $@)
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ $<

# Rule to build each ARM64 binary
bin/arm64/%: cmd/%/main.go
	@echo "Building $@"
	@mkdir -p $(dir $@)
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $@ $<

# Clean the bin directory
clean:
	@echo "Cleaning the bin directory"
	@rm -rf bin
