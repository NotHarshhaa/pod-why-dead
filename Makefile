.PHONY: build install clean test lint run

BINARY_NAME=pod-why-dead
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME) .

install:
	go install -ldflags "$(LDFLAGS)" .

clean:
	rm -rf bin/
	go clean

test:
	go test -v ./...

lint:
	golangci-lint run ./...

run:
	go run . $(ARGS)
