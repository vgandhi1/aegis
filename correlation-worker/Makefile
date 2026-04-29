.PHONY: build test race lint

build:
	go build -o bin/aegis-correlation ./cmd/worker

test:
	go test ./...

race:
	go test -race ./...

lint:
	golangci-lint run ./... 2>/dev/null || staticcheck ./... 2>/dev/null || go vet ./...
