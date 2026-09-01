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

func StartScheduler(ctx context.Context, client *whatsmeow.Client, service *Service, store storage.Store) {
	ticker := time.NewTicker(service.GetInterval())

	go func() {
		defer ticker.Stop()

		checkAndNotify := func() {
			event, err := service.GetLatest(ctx)
			if err != nil {
				logutil.Error(ctx, "BMKG fetch error", "error", err)
				return
			}
			if event == nil {
				return
			}

			if event.ID() == "" {
				return
			}

			eventTime, ok := event.EventTime()
			if !ok {
				logutil.Warn(ctx, "Error parsing earthquake time", "datetime", event.DateTime)
				eventTime = time.Now()
			}

			meetsThreshold := service.MeetsThreshold(event.Magnitude)
			storedEvent := event.ToStorageEvent(eventTime, meetsThreshold)

			isNew, err := store.SaveEarthquakeEvent(ctx, storedEvent)
			if err != nil {
				logutil.Error(ctx, "Error saving earthquake event", "error", err, "id", storedEvent.ID)
				return
			}
			if !isNew {
				// Already seen (and, if applicable, already notified) on a previous tick.
				return
			}
			if !meetsThreshold {
				return
			}
			if time.Since(eventTime) > (2 * service.GetInterval()) {
				logutil.Info(ctx, "Skipping stale earthquake event", "eventTime", eventTime)
				return
			}

			subscribers, err := store.ListEarthquakeSubscriptions(ctx)
			if err != nil {
				logutil.Error(ctx, "Error getting earthquake subscribers", "error", err)
				return
			}
			if len(subscribers) == 0 {
				return
			}

			var shakemap []byte
			if storedEvent.ShakeMap != "" {
				if img, err := service.GetShakeMap(ctx, storedEvent.ShakeMap); err == nil && len(img) > 0 {
					shakemap = img
				}
			}

			text := FormatEvent(*storedEvent)
			notifiedAny := false
			for _, jidStr := range subscribers {
				jid, err := types.ParseJID(jidStr)
				if err != nil {
					logutil.Error(ctx, "Invalid earthquake subscriber JID", "jid", jidStr, "error", err)
					continue
				}

				var sendErr error
				if len(shakemap) > 0 {
					sendErr = helper.SendImageMessageWithCaption(ctx, client, jid, &shakemap, text, nil)
				} else {
					helper.SendTextMessage(ctx, client, jid, text, nil)
				}
				if sendErr != nil {
					logutil.Error(ctx, "Error sending earthquake alert", "error", sendErr, "jid", jidStr)
					continue
				}
				notifiedAny = true
			}

			if notifiedAny {
				if err := store.MarkEarthquakeNotified(ctx, storedEvent.ID); err != nil {
					logutil.Error(ctx, "Error marking earthquake event notified", "error", err, "id", storedEvent.ID)
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
