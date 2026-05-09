package minecraft

import (
	"context"
	"crazyzbot-go/internal/services"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

type serverTapServerResp struct {
	Motd       string  `json:"motd"`
	Version    string  `json:"version"`
	MaxPlayers float64 `json:"maxPlayers"` // Some APIs return numbers as float
}

type serverTapPlayer struct {
	DisplayName string `json:"displayName"`
}

func (s *Service) doRequest(ctx context.Context, path string, target interface{}) error {
	fullURL, err := url.JoinPath(s.apiURL, path)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return err
	}

	if s.apiKey != "" {
		req.Header.Set("Key", s.apiKey)
		req.Header.Set("X-Api-Key", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API %s returned status: %d", path, resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

// GetServerTapStatus fetches status from a custom API endpoint
func (s *Service) GetServerTapStatus(ctx context.Context) (*services.MinecraftStatus, error) {
	if s.apiURL == "" {
		return nil, fmt.Errorf("MC_API_URL is not configured")
	}

	var serverResp serverTapServerResp
	if err := s.doRequest(ctx, "/v1/server", &serverResp); err != nil {
		return nil, fmt.Errorf("failed to get server info: %w", err)
	}

	var playersResp []serverTapPlayer
	if err := s.doRequest(ctx, "/v1/players", &playersResp); err != nil {
		return nil, fmt.Errorf("failed to get players info: %w", err)
	}

	var playerNames []string
	for _, p := range playersResp {
		playerNames = append(playerNames, p.DisplayName)
	}

	return &services.MinecraftStatus{
		Online:        true,
		Description:   serverResp.Motd,
		PlayersOnline: len(playersResp),
		PlayersMax:    int(serverResp.MaxPlayers),
		Players:       playerNames,
		Version:       serverResp.Version,
		Latency:       0,
	}, nil
}
