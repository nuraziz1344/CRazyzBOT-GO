package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// InvidiousSearchResult represents a single result from the Invidious search API.
type InvidiousSearchResult struct {
	VideoID       string  `json:"videoId"`
	Title         string  `json:"title"`
	Author        string  `json:"author"`
	LengthSeconds float64 `json:"lengthSeconds"`
	ViewCount     int64   `json:"viewCount"`
}

var defaultInvidiousInstances = []string{
	"https://invidious.materialio.us",
	"https://invidious.epicsite.xyz",
}

// getInvidiousInstances returns the list of Invidious instances to try.
// Can be overridden via INVIDIOUS_INSTANCES env var (comma-separated URLs).
func getInvidiousInstances() []string {
	env := os.Getenv("INVIDIOUS_INSTANCES")
	if env == "" {
		return defaultInvidiousInstances
	}
	parts := strings.Split(env, ",")
	instances := make([]string, 0, len(parts))
	for _, p := range parts {
		inst := strings.TrimSpace(p)
		if inst != "" {
			instances = append(instances, inst)
		}
	}
	if len(instances) == 0 {
		return defaultInvidiousInstances
	}
	return instances
}

// SearchInvidious searches YouTube via Invidious API (federated frontend).
// Falls back through multiple instances if one fails.
func (s *Service) SearchInvidious(ctx context.Context, query string, limit int) ([]Metadata, error) {
	instances := getInvidiousInstances()

	for _, instance := range instances {
		results, err := s.searchInvidiousInstance(ctx, instance, query, limit)
		if err == nil {
			return results, nil
		}
		// Log and try next instance
		continue
	}

	return nil, fmt.Errorf("all Invidious instances failed")
}

func (s *Service) searchInvidiousInstance(ctx context.Context, instance string, query string, limit int) ([]Metadata, error) {
	u, _ := url.Parse(instance + "/api/v1/search")
	q := u.Query()
	q.Set("q", query)
	q.Set("type", "video")
	q.Set("sort", "relevance")
	q.Set("fields", "videoId,title,author,lengthSeconds,viewCount")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Invidious %s returned HTTP %d", instance, resp.StatusCode)
	}

	var invidiousResults []InvidiousSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&invidiousResults); err != nil {
		return nil, err
	}

	if len(invidiousResults) == 0 {
		return nil, nil
	}

	// Limit results
	if limit > 0 && len(invidiousResults) > limit {
		invidiousResults = invidiousResults[:limit]
	}

	// Map to Metadata
	results := make([]Metadata, 0, len(invidiousResults))
	for _, r := range invidiousResults {
		webpage := "https://youtu.be/" + r.VideoID
		results = append(results, Metadata{
			ID:       r.VideoID,
			Title:    r.Title,
			Duration: math.Floor(r.LengthSeconds),
			Uploader: r.Author,
			Webpage:  webpage,
		})
	}

	return results, nil
}
