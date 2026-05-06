package gempa

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultEndpoint   = "https://data.bmkg.go.id/DataMKG/TEWS/autogempa.json"
	defaultInterval   = 45 * time.Second
	stateKeyLastEvent = "last_event_id"
)

type AutoGempa struct {
	Infogempa struct {
		Gempa Gempa `json:"gempa"`
	} `json:"Infogempa"`
}

type Gempa struct {
	Tanggal     string `json:"Tanggal"`
	Jam         string `json:"Jam"`
	DateTime    string `json:"DateTime"`
	Coordinates string `json:"Coordinates"`
	Lintang     string `json:"Lintang"`
	Bujur       string `json:"Bujur"`
	Magnitude   string `json:"Magnitude"`
	Kedalaman   string `json:"Kedalaman"`
	Wilayah     string `json:"Wilayah"`
	Potensi     string `json:"Potensi"`
	Dirasakan   string `json:"Dirasakan"`
	Shakemap    string `json:"Shakemap"`
}

type Store struct{ db *sql.DB }

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS gempa_alert_state (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Get(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM gempa_alert_state WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}
func (s *Store) Set(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO gempa_alert_state (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP;
	`, key, value)
	return err
}

type Fetcher struct {
	httpClient   *http.Client
	store        *Store
	endpoint     string
	etag         string
	interval     time.Duration
	lastModified string
	lastEventID  string
}

type Alert struct {
	Text        string
	ShakemapURL string
	Shakemap    []byte
}

type Config struct {
	Endpoint string
	Interval time.Duration
}

func NewFetcher(s *Store, cfg Config) *Fetcher {
	interval := cfg.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	endpoint := cfg.Endpoint
	if strings.TrimSpace(endpoint) == "" {
		endpoint = defaultEndpoint
	}
	return &Fetcher{
		httpClient: &http.Client{Timeout: 7 * time.Second},
		store:      s,
		endpoint:   endpoint,
		interval:   interval,
	}
}

func (f *Fetcher) loadState(ctx context.Context) error {
	id, err := f.store.Get(ctx, stateKeyLastEvent)
	if err != nil {
		return err
	}
	f.lastEventID = id
	return nil
}

func (f *Fetcher) rememberState(ctx context.Context, id string) error {
	f.lastEventID = id
	return f.store.Set(ctx, stateKeyLastEvent, id)
}

func (f *Fetcher) fetchOnce(ctx context.Context) (*AutoGempa, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.endpoint, nil)
	if err != nil {
		return nil, err
	}
	if f.etag != "" {
		req.Header.Set("If-None-Match", f.etag)
	}
	if f.lastModified != "" {
		req.Header.Set("If-Modified-Since", f.lastModified)
	}
	response, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		return nil, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %s", response.Status)
	}
	if et := response.Header.Get("ETag"); et != "" {
		f.etag = et
	}
	if lm := response.Header.Get("Last-Modified"); lm != "" {
		f.lastModified = lm
	}

	var ag AutoGempa
	if err := json.NewDecoder(response.Body).Decode(&ag); err != nil {
		return nil, err
	}
	return &ag, nil
}

func (f *Fetcher) Run(ctx context.Context, handler func(Alert)) error {
	_ = f.loadState(ctx)
	_ = f.tick(ctx, handler)

	t := time.NewTicker(f.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			_ = f.tick(ctx, handler)
		}
	}
}

func (f *Fetcher) tick(ctx context.Context, handler func(Alert)) error {
	ag, err := f.fetchOnce(ctx)
	if err != nil || ag == nil {
		return err
	}

	g := ag.Infogempa.Gempa
	id := fmt.Sprintf("%s|%s|%s", g.DateTime, g.Magnitude, g.Coordinates)
	if id == "" || id == f.lastEventID {
		return nil
	}

	if err := f.rememberState(ctx, id); err != nil {
		return err
	}
	alert := Alert{Text: formatMessage(g), ShakemapURL: buildShakemapURL(g.Shakemap)}
	if alert.ShakemapURL != "" {
		if img, err := f.downloadShakemap(ctx, alert.ShakemapURL); err == nil && len(img) > 0 {
			alert.Shakemap = img
		}
	}

	handler(alert)
	return nil
}

func formatMessage(g Gempa) string {
	ts := g.DateTime
	if t, err := time.Parse(time.RFC3339, g.DateTime); err == nil {
		if loc, errLoc := time.LoadLocation("Asia/Jakarta"); errLoc == nil {
			ts = t.In(loc).Format("02/01/2006 15:04:05")
		}
	}

	lines := []string{
		"*Gempa Terkini*",
		"",
		fmt.Sprintf("*Magnitude:* %s", g.Magnitude),
		fmt.Sprintf("*Waktu:* %s", ts),
		fmt.Sprintf("*Wilayah:* %s", g.Wilayah),
		fmt.Sprintf("*Kedalaman:* %s", g.Kedalaman),
		fmt.Sprintf("*Potensi Tsunami:* %s", g.Potensi),
		"",
		"*Sumber:* BMKG",
	}
	return strings.Join(lines, "\n")
}

func buildShakemapURL(name string) string {
	if strings.TrimSpace(name) == "" {
		return ""
	}
	return fmt.Sprintf("https://static.bmkg.go.id/%s", strings.TrimPrefix(name, "/"))
}

func (f *Fetcher) downloadShakemap(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("shakemap status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}
