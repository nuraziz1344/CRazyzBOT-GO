package commands

import (
	"context"
	"fmt"
	"strings"

	"bot/internal/dto"
	"bot/internal/helper"
	"bot/internal/services"

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

func (h *UtilityHandler) HandleResi(c *whatsmeow.Client, msg *dto.ParsedMsg, args string) {
	parts := strings.Split(args, " ")
	if len(parts) < 2 {
		helper.SendTextMessage(c, msg.From, "Usage: /cekresi <courier> <awb>", nil)
		return
	}

	courier := parts[0]
	awb := parts[1]

	res, err := h.shippingService.CheckResi(context.Background(), courier, awb)
	if err != nil {
		helper.SendTextMessage(c, msg.From, "Error: "+err.Error(), nil)
		return
	}

	reply := fmt.Sprintf("Resi: %s (%s)\nStatus: %s\n\nHistory:\n",
		res.AWB, res.Courier, res.Status)

	for _, h := range res.History {
		reply += fmt.Sprintf("- %s: %s\n", h.Date, h.Description)
	}

	helper.SendTextMessage(c, msg.From, reply, nil)
}
