.PHONY: build run test

build:
	go build -o bin/aegis-mes ./cmd/mes

run: build
	./bin/aegis-mes

test:
	go test ./...
