.PHONY: build run test

BINARY := bin/inference-worker

build:
	go build -o $(BINARY) ./cmd/worker/...

run:
	go run ./cmd/worker/...

test:
	go test ./...
