VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test clean

build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o cheapshot ./cmd/cheapshot

test:
	go test ./...

clean:
	rm -f cheapshot
