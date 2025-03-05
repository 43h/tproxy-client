# Makefile for Go project with build tags

# Variables
BINARY_NAME := client

all: clean
	go build -o $(BINARY_NAME) .

debug:
	go build -gcflags "all=-N -l" -tags debug -o $(BINARY_NAME) .

clean:
	rm -f $(BINARY_NAME)