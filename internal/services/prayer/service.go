package prayer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bot/internal/services"
)

type CityResponse struct {
	Status bool   `json:"status"`
	Data   []City `json:"data"`
}

type City struct {
	ID     string `json:"id"`
	Lokasi string `json:"lokasi"`
}

type ScheduleResponse struct {
	Status bool `json:"status"`
	Data   struct {
		Jadwal Schedule `json:"jadwal"`
	} `json:"data"`
}

type Schedule struct {
	Imsak   string `json:"imsak"`
	Subuh   string `json:"subuh"`
	Dzuhur  string `json:"dzuhur"`
	Ashar   string `json:"ashar"`
	Maghrib string `json:"maghrib"`
	Isya    string `json:"isya"`
	Tanggal string `json:"tanggal"`
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) GetCityID(ctx context.Context, city string) (string, error) {
	resp, err := http.Get("https://api.myquran.com/v2/sholat/kota/cari/" + city)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res CityResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if len(res.Data) == 0 {
		return "", fmt.Errorf("city not found")
	}
	return res.Data[0].ID, nil
}

func (s *Service) SearchCity(ctx context.Context, keyword string) ([]services.City, error) {
	resp, err := http.Get("https://api.myquran.com/v2/sholat/kota/cari/" + keyword)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res CityResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	
	var cities []services.City
	for _, c := range res.Data {
		cities = append(cities, services.City{
			ID:     c.ID,
			Lokasi: c.Lokasi,
		})
	}
	return cities, nil
}

func (s *Service) GetSchedule(ctx context.Context, cityID string) (*services.PrayerSchedule, error) {
	now := time.Now()
	url := fmt.Sprintf("https://api.myquran.com/v2/sholat/jadwal/%s/%d/%d/%d",
		cityID, now.Year(), now.Month(), now.Day())

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res ScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	sched := res.Data.Jadwal
	return &services.PrayerSchedule{
		CityID:  cityID,
		Date:    sched.Tanggal,
		Imsak:   sched.Imsak,
		Fajr:    sched.Subuh,
		Dhuhr:   sched.Dzuhur,
		Asr:     sched.Ashar,
		Maghrib: sched.Maghrib,
		Isha:    sched.Isya,
	}, nil
}
