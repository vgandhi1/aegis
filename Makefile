.PHONY: infra-up infra-down web mes edge correlation inference gateway build-all test-all clean

infra-up:
	cd infra && docker compose up -d

infra-down:
	cd infra && docker compose down

web:
	cd web && npm run dev

mes:
	cd mes-service && go run ./cmd/mes

edge:
	cd edge-gateway && cargo run --release

correlation:
	cd correlation-worker && go run ./cmd/worker

inference:
	cd inference-worker && go run ./cmd/worker

gateway:
	cd api-gateway && go run ./cmd/gateway

build-all:
	make -C correlation-worker build
	make -C mes-service build
	make -C inference-worker build
	make -C api-gateway build
	cargo build --release --manifest-path edge-gateway/Cargo.toml
	npm run build --prefix web

test-all:
	make -C correlation-worker race
	make -C mes-service test
	make -C inference-worker test

clean:
	rm -rf correlation-worker/bin mes-service/bin inference-worker/bin api-gateway/bin
	cargo clean --manifest-path edge-gateway/Cargo.toml
