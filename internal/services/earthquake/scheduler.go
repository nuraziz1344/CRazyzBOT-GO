package earthquake

import (
	"context"
	"log"
	"time"

	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

func StartScheduler(ctx context.Context, client *whatsmeow.Client, service *Service, store storage.SubscriptionStore) {
	ticker := time.NewTicker(service.GetInterval())
	startupTime := time.Now() // Track when scheduler started

	go func() {
		defer ticker.Stop()

		checkAndNotify := func() {
			event, err := service.GetLatest(ctx)
			if err != nil {
				log.Println("BMKG fetch error:", err)
				return
			}

			if event == nil {
				return
			}

			eventID := event.ID()
			if eventID == "" || eventID == service.GetLastEventID() {
				return
			}

			// Parse event time to check if it's recent
			// The DateTime field is in RFC3339 format (e.g., "2026-05-09T07:10:29+07:00")
			eventTime, err := time.Parse(time.RFC3339, event.DateTime)
			if err != nil {
				log.Printf("Error parsing earthquake time '%s': %v", event.DateTime, err)
				// If we can't parse time, still process but log error
				eventTime = time.Now() // Assume recent if parsing fails
			}

			// Only process if:
			// 1. Magnitude > 4.0
			// 2. Event happened after scheduler started (not old data)
			// 3. Event happened within last 2x polling interval (to account for slight delays)
			if event.Magnitude <= 4.0 {
				return
			}
			if eventTime.Before(startupTime) {
				log.Printf("Skipping old earthquake event from %v", eventTime)
				return
			}
			if time.Since(eventTime) > (2 * service.GetInterval()) {
				log.Printf("Skipping earthquake event that's too old: %v", eventTime)
				return
			}

			service.SetLastEventID(eventID)

			alert := Alert{Text: formatMessage(*event), ShakemapURL: buildShakemapURL(event.ShakeMap)}
			if alert.ShakemapURL != "" {
				if img, err := service.GetShakeMap(ctx, event.ShakeMap); err == nil && len(img) > 0 {
					alert.Shakemap = img
				}
			}

			// Get earthquake subscribers from storage
			subscribers, err := store.ListEarthquakeSubscriptions(ctx)
			if err != nil {
				log.Printf("Error getting earthquake subscribers: %v", err)
				return
			}
			if len(subscribers) == 0 {
				return
			}

			for _, jidStr := range subscribers {
				jid := types.NewJID(jidStr, "s.whatsapp.net")
				if len(alert.Shakemap) > 0 {
					helpTextErr := helper.SendImageMessageWithCaption(client, jid, &alert.Shakemap, alert.Text, nil)
					if helpTextErr != nil {
						log.Println("Error sending shakemap:", helpTextErr)
					}
				} else {
					helper.SendTextMessage(client, jid, alert.Text, nil)
				}
			}
		}

		checkAndNotify()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkAndNotify()
			}
		}
	}()
}
