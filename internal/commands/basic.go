package commands

import (
	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
)

func HandlePing(c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	helper.SendTextMessage(c, msg.From, "Pong!", &dto.Quoted{
		QuotedMessage: msg.Message,
		StanzaID:      &msg.StanzaID,
		Participant:   &msg.Participant,
	})
}

func HandleHelp(c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
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
- /tagall [message] - Mention everyone with optional message

*Downloader*
- /yts <query> - Search YouTube
- /ytdl <url> - Download from supported video links
- /ytdl2 <url> - Download YouTube using the scraper provider
- /tiktok (t) <url> - Download TikTok media
- /instagram (ig, igdl) <url> - Download Instagram media
- /facebook (fb, fbdl) <url> - Download Facebook media
- /twitter (x, twitterdl) <url> - Download Twitter/X media

*Utilities*
- /minecraft (mc) [host] - Check Minecraft server status
- /sholat <city> - Get prayer times (e.g., /sholat Jakarta)
- /sholat listkota <keyword> - Search supported prayer cities
- /cekresi <courier> <awb> - Track a shipment

*Prayer & Earthquake*
- /psub <city> - Subscribe to prayer notifications for a city (Example: /prayersubscribe Jakarta)
- /punsub - Unsubscribe from prayer notifications
- /esub - Subscribe to earthquake notifications (magnitude > 4.0)
- /eunsub - Unsubscribe from earthquake notifications

*Auto Features*
- Send a sticker in private chat to auto-convert it with /toimg
- Earthquake alerts and prayer reminders run automatically when configured for subscribed users
`
	helper.SendTextMessage(c, msg.From, helpText, nil)
}
