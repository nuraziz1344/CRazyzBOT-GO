package commands

import (
	"log"
	"os"
	"strings"

	"github.com/nuraziz1344/CRazyzBOT-GO/internal/dto"
	"github.com/nuraziz1344/CRazyzBOT-GO/internal/helper"
	"go.mau.fi/whatsmeow"
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
	case "help":
		helper.SendTextMessage(c, msg.From, "Available commands: /help, /ping, /tagall", nil)
	case "ping":
		helper.SendTextMessage(c, msg.From, "Pong!", &dto.Quoted{
			QuotedMessage: msg.QuotedMessage,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
	case "tagall", "all":
		HandleTagAll(c, msg, args)
	case "s", "sticker":
		HandleSticker(c, msg)
	case "toimg":
		HandleToImg(c, msg)
	default:
		helper.SendTextMessage(c, msg.From, "Unknown command: "+command, &dto.Quoted{
			QuotedMessage: msg.QuotedMessage,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
	}
}
