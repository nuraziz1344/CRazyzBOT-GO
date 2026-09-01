package storage

import (
	"context"
	"fmt"
	"time"
)

const proxyPoolTable = `
	CREATE TABLE IF NOT EXISTS proxy_pool (
		address TEXT PRIMARY KEY,
		country TEXT NOT NULL DEFAULT '',
		source TEXT NOT NULL,
		last_seen TIMESTAMPTZ NOT NULL DEFAULT now(),
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)
`

const proxyPoolLastSeenIndex = `
	CREATE INDEX IF NOT EXISTS idx_proxy_pool_last_seen ON proxy_pool (last_seen DESC)
`

type ProxyRecord struct {
	Address  string
	Country  string
	Source   string
	LastSeen time.Time
}

type ProxyStore interface {
	ReplaceProxyPool(ctx context.Context, proxies []ProxyRecord) error
	LoadProxyPool(ctx context.Context) ([]ProxyRecord, error)
}

func (s *PostgresSubscriptionStore) ReplaceProxyPool(ctx context.Context, proxies []ProxyRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin proxy pool transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM proxy_pool`); err != nil {
		return fmt.Errorf("failed to clear proxy pool: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO proxy_pool (address, country, source, last_seen)
		VALUES ($1, $2, $3, $4)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare proxy pool insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for _, proxy := range proxies {
		lastSeen := proxy.LastSeen
		if lastSeen.IsZero() {
			lastSeen = now
		}
		if _, err := stmt.ExecContext(ctx, proxy.Address, proxy.Country, proxy.Source, lastSeen); err != nil {
			return fmt.Errorf("failed to insert proxy %q: %w", proxy.Address, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit proxy pool: %w", err)
	}
	return nil
}

func (s *PostgresSubscriptionStore) LoadProxyPool(ctx context.Context) ([]ProxyRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT address, country, source, last_seen
		FROM proxy_pool ORDER BY last_seen DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to load proxy pool: %w", err)
	}
	defer rows.Close()

	var proxies []ProxyRecord
	for rows.Next() {
		var proxy ProxyRecord
		if err := rows.Scan(&proxy.Address, &proxy.Country, &proxy.Source, &proxy.LastSeen); err != nil {
			return nil, fmt.Errorf("failed to scan proxy pool row: %w", err)
		}
		proxies = append(proxies, proxy)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating proxy pool: %w", err)
	}
	return proxies, nil
}

var _ ProxyStore = (*PostgresSubscriptionStore)(nil)
