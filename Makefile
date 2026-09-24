BINARY  := ghtidy
# make a version name based on git
VERSION := $(shell git describe --tags --always --dirty)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build run test lint fmt vet

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/ghtidy

run:
	go run ./cmd/ghtidy

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

lint:
	golangci-lint run

