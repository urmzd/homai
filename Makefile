# Set the default shell
SHELL := /bin/bash

# Set the default target
.DEFAULT_GOAL := build

# Find all main.go files in the cmd directory
MAIN_GO_FILES := $(shell find cmd -type f -name main.go)

# Define the binary output files based on the main.go files
BINARIES := $(patsubst cmd/%/main.go,bin/%,$(MAIN_GO_FILES))

# Build AMD64 binaries
build: $(BINARIES)

# Rule to build each ARM64 binary
bin/%: cmd/%/main.go
	@echo "Building $@"
	@mkdir -p $(dir $@)
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ $<

# Clean the bin directory
clean:
	@echo "Cleaning the bin directory"
	@rm -rf bin
