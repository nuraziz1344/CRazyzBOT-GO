package commands

import (
	"log"
	"os"
	"strings"

	"github.com/nuraziz1344/CRazyzBOT-GO/internal/dto"
	"github.com/nuraziz1344/CRazyzBOT-GO/internal/helper"
	"go.mau.fi/whatsmeow"
)

const (
	commandHelp         = "help"
	commandHelpAlias    = "h"
	commandPing         = "ping"
	commandTagAll       = "tagall"
	commandTagAllAlias  = "all"
	commandSticker      = "sticker"
	commandStickerAlias = "s"
	commandToImg        = "toimg"
)

func HandleCommand(c *whatsmeow.Client, msg *dto.ParsedMsg) {
	prefix := os.Getenv("COMMAND_PREFIX")
	if prefix == "" {
		prefix = "/"
	}

	if (msg.Body == "@all" || msg.Body == "@everyone") && msg.GroupInfo != nil {
		HandleTagAll(c, msg, msg.Body)
		return
	}

	if !msg.IsGroup && msg.QuotedMessage == nil && (msg.MediaType == dto.MediaSticker || msg.MediaType == dto.MediaAnimatedSticker) {
		HandleToImg(c, msg)
		return
	}

	if msg.Body == "" || msg.Body[0] != prefix[0] {
		return
	}

	// Parse the command and arguments
	commandParts := strings.SplitN(msg.Body[1:], " ", 2)
	command := commandParts[0]
	args := ""
	if len(commandParts) > 1 {
		args = commandParts[1]
	}

	log.Println("Received command:", command, "with args:", args)

	// Handle the command based on its type
	switch command {
	case commandHelp, commandHelpAlias:
		helper.SendTextMessage(c, msg.From, buildHelpMessage(prefix), nil)
	case commandPing:
		helper.SendTextMessage(c, msg.From, "Pong!", &dto.Quoted{
			QuotedMessage: msg.QuotedMessage,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
	case commandTagAll, commandTagAllAlias:
		HandleTagAll(c, msg, args)
	case commandStickerAlias, commandSticker:
		HandleSticker(c, msg)
	case commandToImg:
		HandleToImg(c, msg)
	default:
		helper.SendTextMessage(c, msg.From, "Unknown command: "+command, &dto.Quoted{
			QuotedMessage: msg.QuotedMessage,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
	}
}

func buildHelpMessage(prefix string) string {
	return strings.Join([]string{
		"Available commands:",
		prefix + commandHelp + ", " + prefix + commandHelpAlias + " - Show this help message",
		prefix + commandPing + " - Check whether the bot is responding",
		prefix + commandTagAll + ", " + prefix + commandTagAllAlias + " - Mention all group members",
		prefix + commandSticker + ", " + prefix + commandStickerAlias + " - Convert image, video, or document media to sticker",
		prefix + commandToImg + " - Convert a sticker to an image or animated output",
	}, "\n")
}
