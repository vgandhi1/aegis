package config

import "os"

type Config struct {
	HTTPAddr       string
	GRPCAddr       string
	NATSURL        string
	ClickHouseHTTP string
	JWTSecret      string
}

func Load() Config {
	return Config{
		HTTPAddr:       getenv("HTTP_ADDR", ":8081"),
		GRPCAddr:       getenv("GRPC_ADDR", ":9090"),
		NATSURL:        getenv("NATS_URL", "nats://127.0.0.1:4222"),
		ClickHouseHTTP: getenv("CLICKHOUSE_HTTP", "http://127.0.0.1:8123"),
		JWTSecret:      getenv("JWT_SECRET", "aegis-dev-secret"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
