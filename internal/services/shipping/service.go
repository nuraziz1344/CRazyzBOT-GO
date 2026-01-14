package shipping

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"bot/internal/services"
)

const defaultApiKey = "b00c3361ef1b2d2ac44bc8979b9698332ddaf4b7e036ea000d7fbe50c1615189"

type Response struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Summary struct {
			Courier string `json:"courier"`
			Status  string `json:"status"`
			Date    string `json:"date"`
			Desc    string `json:"desc"`
		} `json:"summary"`
		History []struct {
			Date string `json:"date"`
			Desc string `json:"desc"`
		} `json:"history"`
	} `json:"data"`
}

type Service struct {
	apiKey string
}

func NewService(apiKey string) *Service {
	if apiKey == "" {
		apiKey = defaultApiKey
	}
	return &Service{apiKey: apiKey}
}

func (s *Service) CheckResi(ctx context.Context, courier, awb string) (*services.ShippingStatus, error) {
	u := fmt.Sprintf("https://api.binderbyte.com/v1/track?api_key=%s&courier=%s&awb=%s",
		s.apiKey, url.QueryEscape(courier), url.QueryEscape(awb))

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res Response
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if res.Status != 200 {
		return nil, fmt.Errorf(res.Message)
	}

	var history []struct {
		Date        string
		Description string
	}
	for _, h := range res.Data.History {
		history = append(history, struct {
			Date        string
			Description string
		}{
			Date:        h.Date,
			Description: h.Desc,
		})
	}

	return &services.ShippingStatus{
		Courier: res.Data.Summary.Courier,
		AWB:     awb,
		Status:  res.Data.Summary.Status,
		History: history,
	}, nil
}
