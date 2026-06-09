package commands

import (
	"context"
	"fmt"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/logutil"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
)

// HandleEarthquakeSubscribe handles the /earthquakesubscribe command
func HandleEarthquakeSubscribe(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	logutil.Info(ctx, "Earthquake subscribe", "jid", msg.From.String())
	if err := store.SetEarthquakeSubscription(ctx, msg.From.String(), true); err != nil {
		logutil.Error(ctx, "Earthquake subscribe failed", "jid", msg.From.String(), "error", err)
		helper.SendTextMessage(ctx, c, msg.From, fmt.Sprintf("Failed to subscribe: %v", err), nil)
		return
	}

	helper.SendTextMessage(ctx, c, msg.From, "✅ Subscribed to earthquake notifications (magnitude > 4.0)", nil)
}

// HandleEarthquakeUnsubscribe handles the /earthquakeunsubscribe command
func HandleEarthquakeUnsubscribe(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	logutil.Info(ctx, "Earthquake unsubscribe", "jid", msg.From.String())
	if err := store.SetEarthquakeSubscription(ctx, msg.From.String(), false); err != nil {
		logutil.Error(ctx, "Earthquake unsubscribe failed", "jid", msg.From.String(), "error", err)
		helper.SendTextMessage(ctx, c, msg.From, fmt.Sprintf("Failed to unsubscribe: %v", err), nil)
		return
	}

	helper.SendTextMessage(ctx, c, msg.From, "✅ Unsubscribed from earthquake notifications", nil)
}
