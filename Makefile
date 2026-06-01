.PHONY: build test lint install clean

build:
	go build -o bin/peekaping-pp-cli ./cmd/peekaping-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/peekaping-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/peekaping-pp-mcp ./cmd/peekaping-pp-mcp

install-mcp:
	go install ./cmd/peekaping-pp-mcp

build-all: build build-mcp
