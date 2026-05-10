package commands

import (
	"context"
	"fmt"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
)

// HandleEarthquakeSubscribe handles the /earthquakesubscribe command
func HandleEarthquakeSubscribe(c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	ctx := context.Background()

	if err := store.SetEarthquakeSubscription(ctx, msg.From.String(), true); err != nil {
		helper.SendTextMessage(c, msg.From, fmt.Sprintf("Failed to subscribe: %v", err), nil)
		return
	}

	helper.SendTextMessage(c, msg.From, "✅ Subscribed to earthquake notifications (magnitude > 4.0)", nil)
}

// HandleEarthquakeUnsubscribe handles the /earthquakeunsubscribe command
func HandleEarthquakeUnsubscribe(c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	ctx := context.Background()

	if err := store.SetEarthquakeSubscription(ctx, msg.From.String(), false); err != nil {
		helper.SendTextMessage(c, msg.From, fmt.Sprintf("Failed to unsubscribe: %v", err), nil)
		return
	}

	helper.SendTextMessage(c, msg.From, "✅ Unsubscribed from earthquake notifications", nil)
}
