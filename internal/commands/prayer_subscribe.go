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
func HandlePrayerSubscribe(c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	if args == "" {
		helper.SendTextMessage(c, msg.From, "Usage: /prayersubscribe <city>\nExample: /prayersubscribe Jakarta", nil)
		return
	}

	ctx := context.Background()

	// First, try to get the city ID from the prayer service to validate the city
	// We need to access the prayer service - let's get it from the client's dependencies
	// For now, we'll store the city name and let the scheduler resolve it
	// In a real implementation, we'd validate the city exists

	// Store subscription with city name (scheduler will resolve to city ID)
	if err := store.SetPrayerSubscription(ctx, msg.From.String(), args); err != nil {
		helper.SendTextMessage(c, msg.From, fmt.Sprintf("Failed to subscribe: %v", err), nil)
		return
	}

	helper.SendTextMessage(c, msg.From, fmt.Sprintf("✅ Subscribed to prayer notifications for: %s", args), nil)
}

// HandlePrayerUnsubscribe handles the /prayerunsubscribe command
func HandlePrayerUnsubscribe(c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	ctx := context.Background()

	if err := store.DeletePrayerSubscription(ctx, msg.From.String()); err != nil {
		helper.SendTextMessage(c, msg.From, fmt.Sprintf("Failed to unsubscribe: %v", err), nil)
		return
	}

	helper.SendTextMessage(c, msg.From, "✅ Unsubscribed from prayer notifications", nil)
}
