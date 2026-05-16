package services

import "context"

type City struct {
	ID     string
	Lokasi string
}

type PrayerSchedule struct {
	CityID  string
	Date    string
	Imsak   string
	Fajr    string
	Dhuhr   string
	Asr     string
	Maghrib string
	Isha    string
}

type PrayerProvider interface {
	GetCityID(ctx context.Context, city string) (string, error)
	SearchCity(ctx context.Context, keyword string) ([]City, error)
	GetSchedule(ctx context.Context, cityID string) (*PrayerSchedule, error)
}

type MinecraftStatus struct {
	Online        bool
	Description   string
	PlayersOnline int
	PlayersMax    int
	Players       []string
	Version       string
	Latency       int
}

type MinecraftProvider interface {
	GetStatus(ctx context.Context, server string) (*MinecraftStatus, error)
	GetServerTapStatus(ctx context.Context) (*MinecraftStatus, error)
}

type ShippingStatus struct {
	Courier string
	AWB     string
	Status  string
	History []struct {
		Date        string
		Description string
	}
}

type ShippingProvider interface {
	CheckResi(ctx context.Context, courier, awb string) (*ShippingStatus, error)
}

type OCRProvider interface {
	ImageToText(ctx context.Context, image []byte) (string, error)
}

type DownloadResult struct {
	Type     string
	URL      string
	Title    string
	MimeType string
	Buffer   []byte
	Caption  string
	Filename string
	Provider string
}

type Metadata struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Duration float64 `json:"duration"`
	Uploader string  `json:"uploader"`
	Webpage  string  `json:"webpage_url"`
}

type Downloader interface {
	Download(ctx context.Context, url string) ([]*DownloadResult, error)
}

type YouTubeProvider interface {
	Downloader
	Search(ctx context.Context, query string, limit int) ([]Metadata, error)
}
