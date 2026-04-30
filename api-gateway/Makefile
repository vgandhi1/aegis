.PHONY: build run proto build-grpc

BINARY := bin/api-gateway

build:
	go build -o $(BINARY) ./cmd/gateway/...

run:
	go run ./cmd/gateway/...

# Generates gRPC stubs from ../../proto/inference.proto.
# Requires: protoc, protoc-gen-go, protoc-gen-go-grpc
proto:
	mkdir -p internal/pb
	protoc \
		--go_out=internal/pb --go_opt=paths=source_relative \
		--go-grpc_out=internal/pb --go-grpc_opt=paths=source_relative \
		-I ../../proto ../../proto/inference.proto

build-grpc: proto
	go build -tags grpc -o $(BINARY)-grpc ./cmd/gateway/...
