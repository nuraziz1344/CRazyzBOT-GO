package commands

import (
	"bot/internal/dto"
	"bot/internal/helper"

	"go.mau.fi/whatsmeow"
)

func HandlePing(c *whatsmeow.Client, msg *dto.ParsedMsg, args string) {
	helper.SendTextMessage(c, msg.From, "Pong!", &dto.Quoted{
		QuotedMessage: msg.Message,
		StanzaID:      &msg.StanzaID,
		Participant:   &msg.Participant,
	})
}

func HandleHelp(c *whatsmeow.Client, msg *dto.ParsedMsg, args string) {
	helpText := `*Available Commands*

*Utility*
• /ping - Check bot status
• /help - Show this menu
• /tagall - Tag everyone in group
• /ocr - Image to text

*Media*
• /sticker (s) - Image/Video to sticker
• /toimg - Sticker to image

*Downloader*
• /yts - YouTube search
• /ytdl - YouTube download
• /tiktok (tt) - TikTok download
• /instagram (ig) - Instagram download
• /facebook (fb) - Facebook download
• /twitter (x) - Twitter/X download

*Tools*
• /minecraft (mc) - Check server status
• /sholat - Prayer times
• /cekresi - Check shipping receipt
`
	helper.SendTextMessage(c, msg.From, helpText, nil)
}
