// Package config loads runtime configuration from environment
// variables. Per the project rules:
//   - no hardcoded credentials or ports anywhere in the codebase,
//   - no default fallback for DATABASE_URL (fail fast),
//   - no init() side effects — call Load() from main.go.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Environment names. Treated as strings rather than an enum to keep
// the surface area small; only main.go inspects them.
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
	EnvTest        = "test"
)

// Config is the fully-resolved application configuration.
type Config struct {
	// DatabaseURL is the libpq-style DSN for PostgreSQL. Required.
	DatabaseURL string
	// Port is the TCP port the HTTP server listens on. Required.
	Port string
	// Env is one of development/production/test. Optional, defaults
	// to development if unset.
	Env string
}

// Load reads configuration from os.Environ. It returns a non-nil
// error if any required value is missing so callers can crash early
// rather than silently start with a broken config.
func Load() (*Config, error) {
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		return nil, errors.New("config: DATABASE_URL is required")
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		return nil, errors.New("config: PORT is required")
	}

	env := strings.TrimSpace(os.Getenv("ENV"))
	if env == "" {
		env = EnvDevelopment
	}
	if !isKnownEnv(env) {
		return nil, fmt.Errorf("config: ENV must be one of development/production/test, got %q", env)
	}

	return &Config{
		DatabaseURL: dbURL,
		Port:        port,
		Env:         env,
	}, nil
}

func isKnownEnv(e string) bool {
	switch e {
	case EnvDevelopment, EnvProduction, EnvTest:
		return true
	default:
		return false
	}
}
