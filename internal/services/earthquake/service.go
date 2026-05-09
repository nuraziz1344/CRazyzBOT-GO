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
)

const (
	defaultEndpoint  = "https://data.bmkg.go.id/DataMKG/TEWS/autogempa.json"
	defaultInterval  = 45 * time.Second
	bmkgShakeMapBase = "https://data.bmkg.go.id/DataMKG/TEWS/"
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
	ShakeMap    string
}

func (e *Event) ID() string {
	if e.DateTime != "" {
		return strings.TrimSpace(e.DateTime)
	}
	return strings.TrimSpace(e.Tanggal) + "|" + strings.TrimSpace(e.Jam) + "|" + strings.TrimSpace(e.Coordinates) + "|" + fmt.Sprintf("%.1f", e.Magnitude)
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

type Service struct {
	httpClient   *http.Client
	endpoint     string
	interval     time.Duration
	etag         string
	lastModified string
	lastEventID  string
}

func NewService() *Service {
	return &Service{
		httpClient: &http.Client{Timeout: 7 * time.Second},
		endpoint:   defaultEndpoint,
		interval:   defaultInterval,
	}
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
	return &Service{
		httpClient: &http.Client{Timeout: 7 * time.Second},
		endpoint:   endpoint,
		interval:   interval,
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

func (s *Service) GetLastEventID() string {
	return s.lastEventID
}

func (s *Service) SetLastEventID(id string) {
	s.lastEventID = id
}

func formatMessage(g Event) string {
	ts := g.DateTime
	if t, err := time.Parse(time.RFC3339, g.DateTime); err == nil {
		if loc, errLoc := time.LoadLocation("Asia/Jakarta"); errLoc == nil {
			ts = t.In(loc).Format("02/01/2006 15:04:05")
		}
	}

	lines := []string{
		"*Gempa Terkini*",
		"",
		fmt.Sprintf("*Magnitude:* %s", fmt.Sprintf("%.1f", g.Magnitude)),
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
