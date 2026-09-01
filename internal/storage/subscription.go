package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// SubscriptionStore defines the interface for subscription persistence
type SubscriptionStore interface {
	// Prayer subscriptions
	GetPrayerSubscription(ctx context.Context, jid string) (string, error) // returns cityID
	SetPrayerSubscription(ctx context.Context, jid, cityID string) error
	DeletePrayerSubscription(ctx context.Context, jid string) error
	ListPrayerSubscriptions(ctx context.Context) (map[string]string, error) // jid -> cityID

	// Earthquake subscriptions
	IsEarthquakeSubscribed(ctx context.Context, jid string) (bool, error)
	SetEarthquakeSubscription(ctx context.Context, jid string, subscribed bool) error
	ListEarthquakeSubscriptions(ctx context.Context) ([]string, error) // jids

	Close() error
}

// PostgresSubscriptionStore implements SubscriptionStore using PostgreSQL
type PostgresSubscriptionStore struct {
	db *sql.DB
}

// NewPostgresSubscriptionStore creates a new Postgres-backed subscription store.
// dsn is a standard Postgres connection string, e.g.
// "postgres://user:pass@host:5432/dbname?sslmode=disable".
func NewPostgresSubscriptionStore(ctx context.Context, dsn string) (*PostgresSubscriptionStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open subscription database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping subscription database: %w", err)
	}

	if err := createTables(ctx, db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return &PostgresSubscriptionStore{db: db}, nil
}

// createTables creates the necessary tables if they don't exist
func createTables(ctx context.Context, db *sql.DB) error {
	prayerTable := `
		CREATE TABLE IF NOT EXISTS prayer_subscriptions (
			jid TEXT PRIMARY KEY,
			city_id TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`

	earthquakeTable := `
		CREATE TABLE IF NOT EXISTS earthquake_subscriptions (
			jid TEXT PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`

	if _, err := db.ExecContext(ctx, prayerTable); err != nil {
		return fmt.Errorf("failed to create prayer_subscriptions table: %w", err)
	}

	if _, err := db.ExecContext(ctx, earthquakeTable); err != nil {
		return fmt.Errorf("failed to create earthquake_subscriptions table: %w", err)
	}

	if _, err := db.ExecContext(ctx, earthquakeEventsTable); err != nil {
		return fmt.Errorf("failed to create earthquake_events table: %w", err)
	}

	if _, err := db.ExecContext(ctx, earthquakeEventsIndex); err != nil {
		return fmt.Errorf("failed to create earthquake_events index: %w", err)
	}

	if _, err := db.ExecContext(ctx, proxyPoolTable); err != nil {
		return fmt.Errorf("failed to create proxy_pool table: %w", err)
	}
	if _, err := db.ExecContext(ctx, proxyPoolLastSeenIndex); err != nil {
		return fmt.Errorf("failed to create proxy_pool index: %w", err)
	}

	return nil
}

// GetPrayerSubscription retrieves a user's prayer subscription
func (s *PostgresSubscriptionStore) GetPrayerSubscription(ctx context.Context, jid string) (string, error) {
	var cityID string
	err := s.db.QueryRowContext(ctx, `
		SELECT city_id FROM prayer_subscriptions WHERE jid = $1
	`, jid).Scan(&cityID)

	if err == sql.ErrNoRows {
		return "", nil // Not subscribed
	}
	if err != nil {
		return "", fmt.Errorf("failed to get prayer subscription: %w", err)
	}

	return cityID, nil
}

// SetPrayerSubscription saves or updates a user's prayer subscription
func (s *PostgresSubscriptionStore) SetPrayerSubscription(ctx context.Context, jid, cityID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO prayer_subscriptions (jid, city_id, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (jid) DO UPDATE SET city_id = EXCLUDED.city_id, updated_at = now()
	`, jid, cityID)
	if err != nil {
		return fmt.Errorf("failed to set prayer subscription: %w", err)
	}

	return nil
}

// DeletePrayerSubscription removes a user's prayer subscription
func (s *PostgresSubscriptionStore) DeletePrayerSubscription(ctx context.Context, jid string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM prayer_subscriptions WHERE jid = $1
	`, jid)
	if err != nil {
		return fmt.Errorf("failed to delete prayer subscription: %w", err)
	}

	return nil
}

// ListPrayerSubscriptions returns all prayer subscriptions
func (s *PostgresSubscriptionStore) ListPrayerSubscriptions(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT jid, city_id FROM prayer_subscriptions
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list prayer subscriptions: %w", err)
	}
	defer rows.Close()

	subscriptions := make(map[string]string)
	for rows.Next() {
		var jid, cityID string
		if err := rows.Scan(&jid, &cityID); err != nil {
			return nil, fmt.Errorf("failed to scan prayer subscription: %w", err)
		}
		subscriptions[jid] = cityID
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating prayer subscriptions: %w", err)
	}

	return subscriptions, nil
}

// IsEarthquakeSubscribed checks if a user is subscribed to earthquake notifications
func (s *PostgresSubscriptionStore) IsEarthquakeSubscribed(ctx context.Context, jid string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM earthquake_subscriptions WHERE jid = $1)
	`, jid).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check earthquake subscription: %w", err)
	}

	return exists, nil
}

// SetEarthquakeSubscription saves or updates a user's earthquake subscription
func (s *PostgresSubscriptionStore) SetEarthquakeSubscription(ctx context.Context, jid string, subscribed bool) error {
	if subscribed {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO earthquake_subscriptions (jid, updated_at)
			VALUES ($1, now())
			ON CONFLICT (jid) DO UPDATE SET updated_at = now()
		`, jid)
		if err != nil {
			return fmt.Errorf("failed to set earthquake subscription: %w", err)
		}
	} else {
		_, err := s.db.ExecContext(ctx, `
			DELETE FROM earthquake_subscriptions WHERE jid = $1
		`, jid)
		if err != nil {
			return fmt.Errorf("failed to unset earthquake subscription: %w", err)
		}
	}

	return nil
}

// ListEarthquakeSubscriptions returns all earthquake subscribers
func (s *PostgresSubscriptionStore) ListEarthquakeSubscriptions(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT jid FROM earthquake_subscriptions
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list earthquake subscriptions: %w", err)
	}
	defer rows.Close()

	var subscribers []string
	for rows.Next() {
		var jid string
		if err := rows.Scan(&jid); err != nil {
			return nil, fmt.Errorf("failed to scan earthquake subscription: %w", err)
		}
		subscribers = append(subscribers, jid)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating earthquake subscriptions: %w", err)
	}

	return subscribers, nil
}

// Close closes the database connection
func (s *PostgresSubscriptionStore) Close() error {
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return fmt.Errorf("failed to close database: %w", err)
		}
	}

	return nil
}
