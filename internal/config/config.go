package config

import (
	"os"
)

type Config struct {
	SessionFile        string
	LogLevel           string
	CommandPrefix      string
	SubscriptionDBFile string
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

	return &Config{
		SessionFile:        sessionFile,
		LogLevel:           logLevel,
		CommandPrefix:      prefix,
		SubscriptionDBFile: subscriptionDB,
	}
}
