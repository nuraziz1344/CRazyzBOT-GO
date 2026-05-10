package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/mattn/go-sqlite3"
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

// SQLiteSubscriptionStore implements SubscriptionStore using SQLite
type SQLiteSubscriptionStore struct {
	db             *sql.DB
	prayerStmt     *sql.Stmt
	earthquakeStmt *sql.Stmt
	mu             sync.RWMutex
}

// NewSQLiteSubscriptionStore creates a new SQLite-backed subscription store
func NewSQLiteSubscriptionStore(dbPath string) (*SQLiteSubscriptionStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open subscription database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping subscription database: %w", err)
	}

	// Create tables if they don't exist
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	// Prepare statements
	prayerStmt, err := db.Prepare(`
		INSERT OR REPLACE INTO prayer_subscriptions (jid, city_id, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare prayer statement: %w", err)
	}

	earthquakeStmt, err := db.Prepare(`
		INSERT OR REPLACE INTO earthquake_subscriptions (jid, updated_at)
		VALUES (?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare earthquake statement: %w", err)
	}

	return &SQLiteSubscriptionStore{
		db:             db,
		prayerStmt:     prayerStmt,
		earthquakeStmt: earthquakeStmt,
	}, nil
}

// createTables creates the necessary tables if they don't exist
func createTables(db *sql.DB) error {
	// Prayer subscriptions table
	prayerTable := `
		CREATE TABLE IF NOT EXISTS prayer_subscriptions (
			jid TEXT PRIMARY KEY,
			city_id TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`

	// Earthquake subscriptions table
	earthquakeTable := `
		CREATE TABLE IF NOT EXISTS earthquake_subscriptions (
			jid TEXT PRIMARY KEY,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`

	if _, err := db.Exec(prayerTable); err != nil {
		return fmt.Errorf("failed to create prayer_subscriptions table: %w", err)
	}

	if _, err := db.Exec(earthquakeTable); err != nil {
		return fmt.Errorf("failed to create earthquake_subscriptions table: %w", err)
	}

	return nil
}

// GetPrayerSubscription retrieves a user's prayer subscription
func (s *SQLiteSubscriptionStore) GetPrayerSubscription(ctx context.Context, jid string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var cityID string
	err := s.db.QueryRowContext(ctx, `
		SELECT city_id FROM prayer_subscriptions WHERE jid = ?
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
func (s *SQLiteSubscriptionStore) SetPrayerSubscription(ctx context.Context, jid, cityID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.prayerStmt.ExecContext(ctx, jid, cityID)
	if err != nil {
		return fmt.Errorf("failed to set prayer subscription: %w", err)
	}

	return nil
}

// DeletePrayerSubscription removes a user's prayer subscription
func (s *SQLiteSubscriptionStore) DeletePrayerSubscription(ctx context.Context, jid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		DELETE FROM prayer_subscriptions WHERE jid = ?
	`, jid)
	if err != nil {
		return fmt.Errorf("failed to delete prayer subscription: %w", err)
	}

	return nil
}

// ListPrayerSubscriptions returns all prayer subscriptions
func (s *SQLiteSubscriptionStore) ListPrayerSubscriptions(ctx context.Context) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

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
func (s *SQLiteSubscriptionStore) IsEarthquakeSubscribed(ctx context.Context, jid string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM earthquake_subscriptions WHERE jid = ?)
	`, jid).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check earthquake subscription: %w", err)
	}

	return exists, nil
}

// SetEarthquakeSubscription saves or updates a user's earthquake subscription
func (s *SQLiteSubscriptionStore) SetEarthquakeSubscription(ctx context.Context, jid string, subscribed bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if subscribed {
		_, err := s.earthquakeStmt.ExecContext(ctx, jid)
		if err != nil {
			return fmt.Errorf("failed to set earthquake subscription: %w", err)
		}
	} else {
		_, err := s.db.ExecContext(ctx, `
			DELETE FROM earthquake_subscriptions WHERE jid = ?
		`, jid)
		if err != nil {
			return fmt.Errorf("failed to unset earthquake subscription: %w", err)
		}
	}

	return nil
}

// ListEarthquakeSubscriptions returns all earthquake subscribers
func (s *SQLiteSubscriptionStore) ListEarthquakeSubscriptions(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

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
func (s *SQLiteSubscriptionStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.prayerStmt != nil {
		if err := s.prayerStmt.Close(); err != nil {
			return fmt.Errorf("failed to close prayer statement: %w", err)
		}
	}
	if s.earthquakeStmt != nil {
		if err := s.earthquakeStmt.Close(); err != nil {
			return fmt.Errorf("failed to close earthquake statement: %w", err)
		}
	}
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return fmt.Errorf("failed to close database: %w", err)
		}
	}

	return nil
}
