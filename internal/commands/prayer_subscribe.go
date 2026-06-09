package commands

import (
	"context"
	"fmt"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
)

// HandlePrayerSubscribe handles the /prayersubscribe command
func HandlePrayerSubscribe(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	if args == "" {
		helper.SendTextMessage(ctx, c, msg.From, "Usage: /prayersubscribe <city>\nExample: /prayersubscribe Jakarta", nil)
		return
	}

	// Store subscription with city name (scheduler will resolve to city ID)
	if err := store.SetPrayerSubscription(ctx, msg.From.String(), args); err != nil {
		helper.SendTextMessage(ctx, c, msg.From, fmt.Sprintf("Failed to subscribe: %v", err), nil)
		return
	}

	helper.SendTextMessage(ctx, c, msg.From, fmt.Sprintf("✅ Subscribed to prayer notifications for: %s", args), nil)
}

// HandlePrayerUnsubscribe handles the /prayerunsubscribe command
func HandlePrayerUnsubscribe(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	if err := store.DeletePrayerSubscription(ctx, msg.From.String()); err != nil {
		helper.SendTextMessage(ctx, c, msg.From, fmt.Sprintf("Failed to unsubscribe: %v", err), nil)
		return
	}

	helper.SendTextMessage(ctx, c, msg.From, "✅ Unsubscribed from prayer notifications", nil)
}
