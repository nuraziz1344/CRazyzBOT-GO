package earthquake

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"crazyzbot-go/internal/storage"
)

const (
	defaultEndpoint     = "https://data.bmkg.go.id/DataMKG/TEWS/autogempa.json"
	defaultInterval     = 45 * time.Second
	defaultMinMagnitude = 3.5
	bmkgShakeMapBase    = "https://data.bmkg.go.id/DataMKG/TEWS/"
)

type autoGempaResponse struct {
	InfoGempa struct {
		Gempa struct {
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
		} `json:"gempa"`
	} `json:"Infogempa"`
}

type Event struct {
	Tanggal     string
	Jam         string
	DateTime    string
	Coordinates string
	Lintang     string
	Bujur       string
	Magnitude   float64
	Kedalaman   string
	Wilayah     string
	Potensi     string
	Dirasakan   string
	ShakeMap    string // BMKG shakemap filename, e.g. "20260828160955.mmi.jpg" — not a URL
}

func (e *Event) ID() string {
	if e.DateTime != "" {
		return strings.TrimSpace(e.DateTime)
	}
	return strings.TrimSpace(e.Tanggal) + "|" + strings.TrimSpace(e.Jam) + "|" + strings.TrimSpace(e.Coordinates) + "|" + fmt.Sprintf("%.1f", e.Magnitude)
}

// EventTime parses DateTime into a time.Time, falling back to now if it can't be parsed.
func (e *Event) EventTime() (t time.Time, ok bool) {
	t, err := time.Parse(time.RFC3339, e.DateTime)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// ToStorageEvent maps a polled Event to the persisted storage representation.
func (e *Event) ToStorageEvent(eventTime time.Time, meetsThreshold bool) *storage.EarthquakeEvent {
	return &storage.EarthquakeEvent{
		ID:             e.ID(),
		EventTime:      eventTime,
		Tanggal:        e.Tanggal,
		Jam:            e.Jam,
		Coordinates:    e.Coordinates,
		Lintang:        e.Lintang,
		Bujur:          e.Bujur,
		Magnitude:      e.Magnitude,
		Kedalaman:      e.Kedalaman,
		Wilayah:        e.Wilayah,
		Potensi:        e.Potensi,
		Dirasakan:      e.Dirasakan,
		ShakeMap:       e.ShakeMap,
		MeetsThreshold: meetsThreshold,
	}
}

type Config struct {
	Endpoint     string
	Interval     time.Duration
	MinMagnitude float64
}

type Service struct {
	httpClient   *http.Client
	endpoint     string
	interval     time.Duration
	minMagnitude float64
	etag         string
	lastModified string
}

func NewService() *Service {
	return NewServiceWithConfig(Config{})
}

func NewServiceWithConfig(cfg Config) *Service {
	interval := cfg.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	endpoint := cfg.Endpoint
	if strings.TrimSpace(endpoint) == "" {
		endpoint = defaultEndpoint
	}
	minMagnitude := cfg.MinMagnitude
	if minMagnitude <= 0 {
		minMagnitude = defaultMinMagnitude
	}
	return &Service{
		httpClient:   &http.Client{Timeout: 7 * time.Second},
		endpoint:     endpoint,
		interval:     interval,
		minMagnitude: minMagnitude,
	}
}

func (s *Service) GetLatest(ctx context.Context) (*Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.endpoint, nil)
	if err != nil {
		return nil, err
	}
	if s.etag != "" {
		req.Header.Set("If-None-Match", s.etag)
	}
	if s.lastModified != "" {
		req.Header.Set("If-Modified-Since", s.lastModified)
	}
	response, err := s.httpClient.Do(req)
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
		s.etag = et
	}
	if lm := response.Header.Get("Last-Modified"); lm != "" {
		s.lastModified = lm
	}

	var res autoGempaResponse
	if err := json.NewDecoder(response.Body).Decode(&res); err != nil {
		return nil, err
	}

	magStr := strings.ReplaceAll(res.InfoGempa.Gempa.Magnitude, ",", ".")
	mag, _ := strconv.ParseFloat(strings.TrimSpace(magStr), 64)

	return &Event{
		Tanggal:     res.InfoGempa.Gempa.Tanggal,
		Jam:         res.InfoGempa.Gempa.Jam,
		DateTime:    res.InfoGempa.Gempa.DateTime,
		Coordinates: res.InfoGempa.Gempa.Coordinates,
		Lintang:     res.InfoGempa.Gempa.Lintang,
		Bujur:       res.InfoGempa.Gempa.Bujur,
		Magnitude:   mag,
		Kedalaman:   res.InfoGempa.Gempa.Kedalaman,
		Wilayah:     res.InfoGempa.Gempa.Wilayah,
		Potensi:     res.InfoGempa.Gempa.Potensi,
		Dirasakan:   res.InfoGempa.Gempa.Dirasakan,
		ShakeMap:    res.InfoGempa.Gempa.Shakemap,
	}, nil
}

func (s *Service) GetShakeMap(ctx context.Context, fileName string) ([]byte, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return nil, fmt.Errorf("shakemap file empty")
	}
	url := bmkgShakeMapBase + fileName
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bmkg shakemap http status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func (s *Service) GetInterval() time.Duration {
	return s.interval
}

// MeetsThreshold reports whether magnitude is above the configured minimum for notification.
func (s *Service) MeetsThreshold(magnitude float64) bool {
	return magnitude > s.minMagnitude
}

// FormatEvent renders a persisted earthquake event as a WhatsApp message. Shared between the
// live alert sent by the scheduler and the history listed by /gempa.
func FormatEvent(ev storage.EarthquakeEvent) string {
	ts := ev.EventTime.Format("02/01/2006 15:04:05")
	if loc, err := time.LoadLocation("Asia/Jakarta"); err == nil && !ev.EventTime.IsZero() {
		ts = ev.EventTime.In(loc).Format("02/01/2006 15:04:05")
	}

	lines := []string{
		"*Gempa Terkini*",
		"",
		fmt.Sprintf("*Magnitude:* %.1f", ev.Magnitude),
		fmt.Sprintf("*Waktu:* %s", ts),
		fmt.Sprintf("*Wilayah:* %s", ev.Wilayah),
		fmt.Sprintf("*Kedalaman:* %s", ev.Kedalaman),
		fmt.Sprintf("*Potensi Tsunami:* %s", ev.Potensi),
		"",
		"*Sumber:* BMKG",
	}
	return strings.Join(lines, "\n")
}
