package commands

import (
	"context"
	"fmt"
	"strings"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/logutil"
	"crazyzbot-go/internal/services"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
)

type UtilityHandler struct {
	shippingService services.ShippingProvider
}

func NewUtilityHandler(shippingService services.ShippingProvider) *UtilityHandler {
	return &UtilityHandler{
		shippingService: shippingService,
	}
}

func (h *UtilityHandler) HandleResi(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	logutil.Info(ctx, "Resi command", "args", args, "from", msg.From.String())
	parts := strings.Split(args, " ")
	if len(parts) < 2 {
		helper.SendTextMessage(ctx, c, msg.From, "Usage: /cekresi <courier> <awb>", nil)
		return
	}

	courier := parts[0]
	awb := parts[1]

	res, err := h.shippingService.CheckResi(ctx, courier, awb)
	if err != nil {
		logutil.Error(ctx, "Shipping check failed", "courier", courier, "awb", awb, "error", err)
		helper.SendTextMessage(ctx, c, msg.From, logutil.UserError(ctx), nil)
		return
	}

	reply := fmt.Sprintf("Resi: %s (%s)\nStatus: %s\n\nHistory:\n",
		res.AWB, res.Courier, res.Status)

	for _, h := range res.History {
		reply += fmt.Sprintf("- %s: %s\n", h.Date, h.Description)
	}

	helper.SendTextMessage(ctx, c, msg.From, reply, nil)
}
