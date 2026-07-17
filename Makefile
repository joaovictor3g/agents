VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -ldflags "-X main.version=$(VERSION)"

.PHONY: build install test lint clean

build:
	go build $(LDFLAGS) -o bin/agents ./cmd/agents

install:
	go install $(LDFLAGS) ./cmd/agents

test:
	go test ./...

lint:
	gofmt -l . && go vet ./...

clean:
	rm -rf bin
