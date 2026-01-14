package minecraft

import (
	"bot/internal/services"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Tnze/go-mc/bot"
	"github.com/Tnze/go-mc/chat"
)

type Service struct {
	httpClient *http.Client
	apiURL     string
	apiKey     string
}

func NewService() *Service {
	return &Service{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiURL:     os.Getenv("MC_API_URL"),
		apiKey:     os.Getenv("MC_API_KEY"),
	}
}

// Define structs to match the Server List Ping JSON response
type pingResponse struct {
	Version struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	} `json:"version"`
	Players struct {
		Max    int `json:"max"`
		Online int `json:"online"`
		Sample []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"sample"`
	} `json:"players"`
	Description chat.Message `json:"description"`
	Favicon     string       `json:"favicon"`
}

func (s *Service) GetStatus(ctx context.Context, server string) (*services.MinecraftStatus, error) {
	respBytes, delay, err := bot.PingAndList(server)
	if err != nil {
		return nil, err
	}

	var resp pingResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}

	return &services.MinecraftStatus{
		Online:        true,
		Description:   resp.Description.String(),
		PlayersOnline: resp.Players.Online,
		PlayersMax:    resp.Players.Max,
		Version:       resp.Version.Name,
		Latency:       int(delay.Milliseconds()),
	}, nil
}

// GetServerTapStatus fetches status from a custom API endpoint
func (s *Service) GetServerTapStatus(ctx context.Context) (*services.MinecraftStatus, error) {
	if s.apiURL == "" {
		return nil, fmt.Errorf("MC_API_URL is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", s.apiURL, nil)
	if err != nil {
		return nil, err
	}

	if s.apiKey != "" {
		req.Header.Set("Key", s.apiKey)
		req.Header.Set("X-Api-Key", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	// Parsing the specific response from mc-api.crazyz.my.id
	var apiResp struct {
		Online  bool   `json:"online"`
		Version string `json:"version"`
		Players struct {
			Now int `json:"now"`
			Max int `json:"max"`
		} `json:"players"`
		Motd string `json:"motd"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	return &services.MinecraftStatus{
		Online:        true,
		Description:   apiResp.Motd,
		PlayersOnline: apiResp.Players.Now,
		PlayersMax:    apiResp.Players.Max,
		Version:       apiResp.Version,
		Latency:       0,
	}, nil
}
