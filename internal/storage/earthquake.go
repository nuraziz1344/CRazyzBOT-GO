package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const earthquakeEventsTable = `
	CREATE TABLE IF NOT EXISTS earthquake_events (
		id TEXT PRIMARY KEY,
		event_time TIMESTAMPTZ,
		tanggal TEXT,
		jam TEXT,
		coordinates TEXT,
		lintang TEXT,
		bujur TEXT,
		magnitude DOUBLE PRECISION NOT NULL DEFAULT 0,
		kedalaman TEXT,
		wilayah TEXT,
		potensi TEXT,
		dirasakan TEXT,
		shakemap TEXT,
		meets_threshold BOOLEAN NOT NULL DEFAULT FALSE,
		notified BOOLEAN NOT NULL DEFAULT FALSE,
		notified_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)
`

const earthquakeEventsIndex = `
	CREATE INDEX IF NOT EXISTS idx_earthquake_events_event_time
		ON earthquake_events (event_time DESC)
`

// EarthquakeEvent is a single polled BMKG earthquake reading, persisted so it can be
// deduplicated across restarts and listed back out via /gempa.
type EarthquakeEvent struct {
	ID          string
	EventTime   time.Time
	Tanggal     string
	Jam         string
	Coordinates string
	Lintang     string
	Bujur       string
	Magnitude   float64
	Kedalaman   string
	Wilayah     string
	Potensi     string
	Dirasakan   string
	ShakeMap    string

	MeetsThreshold bool
	Notified       bool
	NotifiedAt     *time.Time
	CreatedAt      time.Time
}

// EarthquakeEventStore persists polled earthquake events and their notification state.
type EarthquakeEventStore interface {
	// SaveEarthquakeEvent inserts ev if its ID hasn't been seen before. isNew is true
	// only when a row was actually inserted, which callers use as the dedup signal.
	SaveEarthquakeEvent(ctx context.Context, ev *EarthquakeEvent) (isNew bool, err error)
	ListRecentEarthquakes(ctx context.Context, limit int) ([]EarthquakeEvent, error)
	MarkEarthquakeNotified(ctx context.Context, id string) error
}

// Store is the full persistence surface handlers and schedulers depend on.
type Store interface {
	SubscriptionStore
	EarthquakeEventStore
	ProxyStore
}

func (s *PostgresSubscriptionStore) SaveEarthquakeEvent(ctx context.Context, ev *EarthquakeEvent) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO earthquake_events (
			id, event_time, tanggal, jam, coordinates, lintang, bujur,
			magnitude, kedalaman, wilayah, potensi, dirasakan, shakemap, meets_threshold
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (id) DO NOTHING
	`,
		ev.ID, ev.EventTime, ev.Tanggal, ev.Jam, ev.Coordinates, ev.Lintang, ev.Bujur,
		ev.Magnitude, ev.Kedalaman, ev.Wilayah, ev.Potensi, ev.Dirasakan, ev.ShakeMap, ev.MeetsThreshold,
	)
	if err != nil {
		return false, fmt.Errorf("failed to save earthquake event: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to check rows affected for earthquake event: %w", err)
	}

	return rows > 0, nil
}

func (s *PostgresSubscriptionStore) ListRecentEarthquakes(ctx context.Context, limit int) ([]EarthquakeEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, event_time, tanggal, jam, coordinates, lintang, bujur,
			magnitude, kedalaman, wilayah, potensi, dirasakan, shakemap,
			meets_threshold, notified, notified_at, created_at
		FROM earthquake_events
		ORDER BY event_time DESC NULLS LAST, created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list earthquake events: %w", err)
	}
	defer rows.Close()

	var events []EarthquakeEvent
	for rows.Next() {
		var ev EarthquakeEvent
		var eventTime sql.NullTime
		if err := rows.Scan(
			&ev.ID, &eventTime, &ev.Tanggal, &ev.Jam, &ev.Coordinates, &ev.Lintang, &ev.Bujur,
			&ev.Magnitude, &ev.Kedalaman, &ev.Wilayah, &ev.Potensi, &ev.Dirasakan, &ev.ShakeMap,
			&ev.MeetsThreshold, &ev.Notified, &ev.NotifiedAt, &ev.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan earthquake event: %w", err)
		}
		if eventTime.Valid {
			ev.EventTime = eventTime.Time
		}
		events = append(events, ev)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating earthquake events: %w", err)
	}

	return events, nil
}

func (s *PostgresSubscriptionStore) MarkEarthquakeNotified(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE earthquake_events SET notified = TRUE, notified_at = now() WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("failed to mark earthquake event notified: %w", err)
	}

	return nil
}
