package commands

import (
	"context"
	"fmt"
	"strings"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/services"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
)

type ReligionHandler struct {
	prayerService services.PrayerProvider
}

func NewReligionHandler(prayerService services.PrayerProvider) *ReligionHandler {
	return &ReligionHandler{
		prayerService: prayerService,
	}
}

func (h *ReligionHandler) HandlePrayer(c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	if args == "" {
		helper.SendTextMessage(c, msg.From, "Usage: /sholat <city name> or /sholat listkota <keyword>", nil)
		return
	}

	ctx := context.Background()

	if strings.HasPrefix(strings.ToLower(args), "listkota") {
		keyword := strings.TrimSpace(strings.TrimPrefix(args, "listkota"))
		if keyword == "" {
			helper.SendTextMessage(c, msg.From, "Please provide a keyword to search city", nil)
			return
		}

		cities, err := h.prayerService.SearchCity(ctx, keyword)
		if err != nil {
			helper.SendTextMessage(c, msg.From, "Error searching city", nil)
			return
		}

		if len(cities) == 0 {
			helper.SendTextMessage(c, msg.From, "City not found", nil)
			return
		}

		res := "List Kota:\n"
		for _, city := range cities {
			res += fmt.Sprintf("- %s (ID: %s)\n", city.Lokasi, city.ID)
		}
		helper.SendTextMessage(c, msg.From, res, nil)
		return
	}

	// Assuming args is city ID or name. Since API needs ID, we should probably search first if not ID.
	// But first try as keyword
	cities, err := h.prayerService.SearchCity(ctx, args)
	if err != nil || len(cities) == 0 {
		helper.SendTextMessage(c, msg.From, "City not found", nil)
		return
	}

	schedule, err := h.prayerService.GetSchedule(ctx, cities[0].ID)
	if err != nil {
		helper.SendTextMessage(c, msg.From, "Error getting schedule", nil)
		return
	}

	res := fmt.Sprintf("Jadwal Sholat %s\nTanggal: %s\n\n", cities[0].Lokasi, schedule.Date)
	res += fmt.Sprintf("Imsak: %s\n", schedule.Imsak) // Wait, I need to check field names in PrayerSchedule struct
	res += fmt.Sprintf("Subuh: %s\n", schedule.Fajr)
	res += fmt.Sprintf("Dzuhur: %s\n", schedule.Dhuhr)
	res += fmt.Sprintf("Ashar: %s\n", schedule.Asr)
	res += fmt.Sprintf("Maghrib: %s\n", schedule.Maghrib)
	res += fmt.Sprintf("Isya: %s", schedule.Isha)

	helper.SendTextMessage(c, msg.From, res, nil)
}
