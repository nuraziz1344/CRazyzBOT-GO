package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/logutil"
	"crazyzbot-go/internal/services"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

type GameHandler struct {
	mcService services.MinecraftProvider
}

func NewGameHandler(mcService services.MinecraftProvider) *GameHandler {
	return &GameHandler{
		mcService: mcService,
	}
}

func (h *GameHandler) HandleMinecraft(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	logutil.Info(ctx, "Minecraft command", "args", args, "from", msg.From.String())
	if args == "" {
		// Default check for authorized groups
		authorized := false
		ownerNumber := os.Getenv("OWNER_NUMBER")

		if ownerNumber != "" && msg.Sender.User == ownerNumber {
			authorized = true
		} else if msg.IsGroup && msg.GroupInfo != nil {
			// Check if group name starts with TCBB
			if strings.HasPrefix(msg.GroupInfo.Name, "TCBB") {
				authorized = true
			} else if ownerNumber != "" {
				// Check if owner is in the group
				for _, participant := range msg.GroupInfo.Participants {
					if participant.PhoneNumber.User == ownerNumber {
						authorized = true
						break
					}
				}
			}
		}

		if authorized {
			status, err := h.mcService.GetServerTapStatus(ctx)
			if err != nil {
				logutil.Error(ctx, "Minecraft server status fetch failed", "error", err)
				helper.SendTextMessage(ctx, c, msg.From, logutil.UserError(ctx), nil)
				return
			}
			h.sendMinecraftStatus(ctx, c, msg.From, "Private Server", status)
			return
		}

		helper.SendTextMessage(ctx, c, msg.From, "Please provide a server IP.\nExample: /mc mc.hypixel.net", nil)
		return
	}

	parts := strings.Split(args, " ")
	ip := parts[0]

	status, err := h.mcService.GetStatus(ctx, ip)
	if err != nil {
		logutil.Error(ctx, "Minecraft server status failed", "ip", ip, "error", err)
		helper.SendTextMessage(ctx, c, msg.From, "Failed to get server status", nil)
		return
	}

	if !status.Online {
		helper.SendTextMessage(ctx, c, msg.From, fmt.Sprintf("Server %s is offline", ip), nil)
		return
	}

	h.sendMinecraftStatus(ctx, c, msg.From, ip, status)
}

func (h *GameHandler) sendMinecraftStatus(ctx context.Context, c *whatsmeow.Client, jid types.JID, name string, status *services.MinecraftStatus) {
	res := fmt.Sprintf("Server Status: %s\n", name)
	res += fmt.Sprintf("Version: %s\n", status.Version)
	res += fmt.Sprintf("Players: %d/%d\n", status.PlayersOnline, status.PlayersMax)
	if status.Latency > 0 {
		res += fmt.Sprintf("Latency: %dms\n", status.Latency)
	}
	if status.Description != "" {
		res += fmt.Sprintf("MOTD: %s\n", status.Description)
	}
	if len(status.Players) > 0 {
		res += fmt.Sprintf("Online Players:\n- %s\n", strings.Join(status.Players, "\n- "))
	}

	helper.SendTextMessage(ctx, c, jid, res, nil)
}
