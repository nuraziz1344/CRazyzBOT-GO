package commands

import (
	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"

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
	helpText := `*CRazyzBOT Features*

*General*
- /help (h) - Show this menu
- /ping - Check bot status

*Media Tools*
- /sticker (s, stiker) - Convert image, video, or document media to sticker
- /sticker2 (s2) - Sticker conversion with stronger compression for large files
- /toimg - Convert sticker to image or GIF output
- /ocr - Extract text from an image

*Group Tools*
- /tagall (all) - Mention everyone in the current group
- @all or @everyone - Trigger tag-all without a slash command

*Downloader*
- /yts <query> - Search YouTube
- /ytdl <url> - Download from supported video links
- /tiktok (tt) <url> - Download TikTok media
- /instagram (ig, igdl) <url> - Download Instagram media
- /facebook (fb, fbdl) <url> - Download Facebook media
- /twitter (x, twitterdl) <url> - Download Twitter/X media

*Utilities*
- /minecraft (mc) [host] - Check Minecraft server status
- /sholat (jadwalsholat) <city> - Get prayer times
- /sholat listkota <keyword> - Search supported prayer cities
- /cekresi <courier> <awb> - Track a shipment

*Auto Features*
- Send a sticker in private chat to auto-convert it with /toimg
- Earthquake alerts and prayer reminders run automatically when configured
`
	helper.SendTextMessage(c, msg.From, helpText, nil)
}
