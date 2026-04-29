.PHONY: infra-up infra-down web mes edge correlation build-all test-all clean

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

build-all:
	make -C correlation-worker build
	make -C mes-service build
	cargo build --release --manifest-path edge-gateway/Cargo.toml
	npm run build --prefix web

test-all:
	make -C correlation-worker race
	make -C mes-service test

clean:
	rm -rf correlation-worker/bin mes-service/bin
	cargo clean --manifest-path edge-gateway/Cargo.toml
