# Makefile for Go project with build tags

# Variables
BINARY_NAME := client

all: clean
	go build -o $(BINARY_NAME) .

# Clean the build
clean:
	rm -f $(BINARY_NAME)

# Run the Go application

.PHONY: build clean