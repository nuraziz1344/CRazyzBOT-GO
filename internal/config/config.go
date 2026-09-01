package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

const defaultEarthquakeMinMagnitude = 3.5

type Config struct {
	PostgresDSN            string
	LogLevel               string
	CommandPrefix          string
	EarthquakeMinMagnitude float64

	// Proxy configuration
	ProxyEnabled         bool
	ProxyCountry         string
	ProxyRefreshInterval time.Duration
}

func LoadConfig() *Config {
	postgresDSN := os.Getenv("POSTGRES_DSN")
	if postgresDSN == "" {
		log.Fatal("POSTGRES_DSN is required")
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "warn"
	}

	prefix := os.Getenv("COMMAND_PREFIX")
	if prefix == "" {
		prefix = "/"
	}

	minMagnitude := defaultEarthquakeMinMagnitude
	if raw := os.Getenv("EARTHQUAKE_MIN_MAGNITUDE"); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed > 0 {
			minMagnitude = parsed
		} else {
			log.Printf("Invalid EARTHQUAKE_MIN_MAGNITUDE %q, using default %.1f", raw, defaultEarthquakeMinMagnitude)
		}
	}

	// Proxy config
	proxyCountry := os.Getenv("PROXY_COUNTRY")
	proxyEnabled := os.Getenv("PROXY_ENABLED")
	if proxyCountry == "" {
		proxyCountry = "ID"
	}

	refreshInterval := 30 * time.Minute
	if val := os.Getenv("PROXY_REFRESH_INTERVAL"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			refreshInterval = d
		}
	}

	return &Config{
		PostgresDSN:            postgresDSN,
		LogLevel:               logLevel,
		CommandPrefix:          prefix,
		EarthquakeMinMagnitude: minMagnitude,
		ProxyEnabled:           proxyEnabled == "true" || proxyEnabled == "1",
		ProxyCountry:           proxyCountry,
		ProxyRefreshInterval:   refreshInterval,
	}
}
