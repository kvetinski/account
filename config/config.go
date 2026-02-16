package config

import (
	"os"
	"strconv"
)

type Config struct {
	GRPCAddr    string
	MetricsAddr string
	PostgresURI string

	TracingEnabled      bool
	TracingServiceName  string
	TracingOTLPEndpoint string
	TracingOTLPInsecure bool
	TracingSampleRatio  float64
}

func New() Config {
	return Config{
		GRPCAddr:    getEnv("GRPC_ADDR", ":8080"),
		MetricsAddr: getEnv("METRICS_ADDR", ":9091"),
		PostgresURI: getEnv("POSTGRES_URI", "postgres://account:account@localhost:5432/account?sslmode=disable"),

		TracingEnabled:      getEnvBool("OTEL_ENABLED", false),
		TracingServiceName:  getEnv("OTEL_SERVICE_NAME", "account-service"),
		TracingOTLPEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		TracingOTLPInsecure: getEnvBool("OTEL_EXPORTER_OTLP_INSECURE", true),
		TracingSampleRatio:  getEnvFloat("OTEL_TRACES_SAMPLER_ARG", 1.0),
	}
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	return v
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}

	return b
}

func getEnvFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}

	if f < 0 || f > 1 {
		return fallback
	}

	return f
}
