package earthquake

import (
	"context"
	"time"

	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/logutil"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

func StartScheduler(ctx context.Context, client *whatsmeow.Client, service *Service, store storage.SubscriptionStore) {
	ticker := time.NewTicker(service.GetInterval())
	startupTime := time.Now()

	go func() {
		defer ticker.Stop()

		checkAndNotify := func() {
			logger := logutil.LoggerFromContext(ctx)
			event, err := service.GetLatest(ctx)
			if err != nil {
				logger.Error("BMKG fetch error", "error", err)
				return
			}
			if event == nil {
				return
			}

			eventID := event.ID()
			if eventID == "" || eventID == service.GetLastEventID() {
				return
			}

			eventTime, err := time.Parse(time.RFC3339, event.DateTime)
			if err != nil {
				logger.Warn("Error parsing earthquake time", "datetime", event.DateTime, "error", err)
				eventTime = time.Now()
			}

			if event.Magnitude <= 4.0 {
				return
			}
			if eventTime.Before(startupTime) {
				logger.Info("Skipping old earthquake event", "eventTime", eventTime)
				return
			}
			if time.Since(eventTime) > (2 * service.GetInterval()) {
				logger.Info("Skipping old earthquake event (interval)", "eventTime", eventTime)
				return
			}

			service.SetLastEventID(eventID)

			alert := Alert{Text: formatMessage(*event), ShakemapURL: buildShakemapURL(event.ShakeMap)}
			if alert.ShakemapURL != "" {
				if img, err := service.GetShakeMap(ctx, event.ShakeMap); err == nil && len(img) > 0 {
					alert.Shakemap = img
				}
			}

			subscribers, err := store.ListEarthquakeSubscriptions(ctx)
			if err != nil {
				logger.Error("Error getting earthquake subscribers", "error", err)
				return
			}
			if len(subscribers) == 0 {
				return
			}

			for _, jidStr := range subscribers {
				jid := types.NewJID(jidStr, "s.whatsapp.net")
				if len(alert.Shakemap) > 0 {
					if sendErr := helper.SendImageMessageWithCaption(ctx, client, jid, &alert.Shakemap, alert.Text, nil); sendErr != nil {
						logger.Error("Error sending shakemap", "error", sendErr, "jid", jidStr)
					}
				} else {
					helper.SendTextMessage(ctx, client, jid, alert.Text, nil)
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
