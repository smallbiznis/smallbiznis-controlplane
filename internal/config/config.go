package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config represents application configuration.
type Config struct {
	Server ServerConfig
	GRPC   GRPCConfig
	TLS    TLSConfig
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// GRPCConfig holds gRPC server configuration.
type GRPCConfig struct {
	Addr string
}

// TLSConfig controls TLS behaviour for the HTTP server.
type TLSConfig struct {
	Enable   bool
	CertPath string
	KeyPath  string
}

// Provide loads configuration from environment variables with sensible defaults.
func Provide() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Addr:         getEnv("SERVER_ADDR", ":8080"),
			ReadTimeout:  getDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
		},
		GRPC: GRPCConfig{
			Addr: getEnv("GRPC_ADDR", ":9090"),
		},
		TLS: TLSConfig{
			Enable:   getBool("TLS_ENABLE", false),
			CertPath: os.Getenv("TLS_CERT_PATH"),
			KeyPath:  os.Getenv("TLS_KEY_PATH"),
		},
	}

	if cfg.TLS.Enable {
		if cfg.TLS.CertPath == "" || cfg.TLS.KeyPath == "" {
			return nil, fmt.Errorf("tls enabled but TLS_CERT_PATH or TLS_KEY_PATH not provided")
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return d
}

func getBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
