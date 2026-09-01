package config

import (
	"os"
	"time"
)

type Config struct {
	SessionFile        string
	LogLevel           string
	CommandPrefix      string
	SubscriptionDBFile string

	// Proxy configuration
	ProxyEnabled       bool
	ProxyCountry       string
	ProxyRefreshInterval time.Duration
}

func LoadConfig() *Config {
	sessionFile := os.Getenv("SESSION_FILE")
	if sessionFile == "" {
		sessionFile = "data/session.db"
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "warn"
	}

	prefix := os.Getenv("COMMAND_PREFIX")
	if prefix == "" {
		prefix = "/"
	}

	subscriptionDB := os.Getenv("SUBSCRIPTION_DB_FILE")
	if subscriptionDB == "" {
		subscriptionDB = "data/subscriptions.db"
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
		SessionFile:          sessionFile,
		LogLevel:             logLevel,
		CommandPrefix:        prefix,
		SubscriptionDBFile:   subscriptionDB,
		ProxyEnabled:         proxyEnabled == "true" || proxyEnabled == "1",
		ProxyCountry:         proxyCountry,
		ProxyRefreshInterval: refreshInterval,
	}
}
