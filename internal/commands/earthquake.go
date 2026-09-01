package commands

import (
	"context"
	"fmt"
	"strings"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/logutil"
	"crazyzbot-go/internal/services/earthquake"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
)

const recentEarthquakeCount = 3

// HandleEarthquake handles the /gempa command: recent earthquake history by default,
// plus subscribe/unsub subcommands for earthquake alert notifications.
func HandleEarthquake(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.Store) {
	logutil.Info(ctx, "Earthquake command", "args", args, "from", msg.From.String())

	switch strings.ToLower(strings.TrimSpace(args)) {
	case "subscribe", "sub":
		handleEarthquakeSubscribe(ctx, c, msg, store)
		return
	case "unsub", "unsubscribe":
		handleEarthquakeUnsubscribe(ctx, c, msg, store)
		return
	}

	events, err := store.ListRecentEarthquakes(ctx, recentEarthquakeCount)
	if err != nil {
		logutil.Error(ctx, "Failed to list recent earthquakes", "error", err)
		helper.SendTextMessage(ctx, c, msg.From, logutil.UserError(ctx), nil)
		return
	}

	if len(events) == 0 {
		helper.SendTextMessage(ctx, c, msg.From, "No earthquake data recorded yet. Try again after the next poll.\n\nUse /gempa subscribe to get notified when a new earthquake is detected.", nil)
		return
	}

	parts := make([]string, 0, len(events))
	for _, ev := range events {
		parts = append(parts, earthquake.FormatEvent(ev))
	}
	text := strings.Join(parts, "\n\n") + "\n\nUse /gempa subscribe to get notified automatically."

	helper.SendTextMessage(ctx, c, msg.From, text, nil)
}

func handleEarthquakeSubscribe(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, store storage.Store) {
	logutil.Info(ctx, "Earthquake subscribe", "jid", msg.From.String())
	if err := store.SetEarthquakeSubscription(ctx, msg.From.String(), true); err != nil {
		logutil.Error(ctx, "Earthquake subscribe failed", "jid", msg.From.String(), "error", err)
		helper.SendTextMessage(ctx, c, msg.From, fmt.Sprintf("Failed to subscribe: %v", err), nil)
		return
	}

	helper.SendTextMessage(ctx, c, msg.From, "✅ Subscribed to earthquake notifications", nil)
}

func handleEarthquakeUnsubscribe(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, store storage.Store) {
	logutil.Info(ctx, "Earthquake unsubscribe", "jid", msg.From.String())
	if err := store.SetEarthquakeSubscription(ctx, msg.From.String(), false); err != nil {
		logutil.Error(ctx, "Earthquake unsubscribe failed", "jid", msg.From.String(), "error", err)
		helper.SendTextMessage(ctx, c, msg.From, fmt.Sprintf("Failed to unsubscribe: %v", err), nil)
		return
	}

	helper.SendTextMessage(ctx, c, msg.From, "✅ Unsubscribed from earthquake notifications", nil)
}
