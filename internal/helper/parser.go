package helper

import (
	"strings"

	"github.com/nuraziz1344/CRazyzBOT-GO/internal/dto"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
)

func GetSenderNumber(sender string) string {
	if strings.Contains(sender, ":") {
		return strings.Split(sender, ":")[0]
	} else {
		return strings.Split(sender, "@")[0]
	}
}

func ParseQuotedMessage(message *waE2E.Message) dto.ParsedMsg {
	var body string
	var media whatsmeow.DownloadableMessage
	var mediaType string
	var mediaFilename string

	if message.ViewOnceMessage != nil {
		message = message.ViewOnceMessage.Message
	} else if message.DocumentWithCaptionMessage != nil {
		message = message.DocumentWithCaptionMessage.Message
	}

	if message.Conversation != nil {
		body = message.GetConversation()
	} else if message.ExtendedTextMessage != nil {
		body = message.ExtendedTextMessage.GetText()
	} else if message.ImageMessage != nil {
		mediaType = "image"
		body = message.ImageMessage.GetCaption()
		media = message.ImageMessage

	} else if message.VideoMessage != nil {
		mediaType = "video"
		body = message.VideoMessage.GetCaption()
		media = message.VideoMessage
	} else if message.DocumentMessage != nil {
		mediaType = "document"
		media = message.DocumentMessage
		mediaFilename = message.DocumentMessage.GetFileName()
		if mediaFilename == "" {
			mediaFilename = message.DocumentMessage.GetTitle()
		}
		if message.DocumentMessage.GetTitle() != message.DocumentMessage.GetCaption() {
			body = message.DocumentMessage.GetCaption()
		}
	} else if message.StickerMessage != nil {
		mediaType = "sticker"
		if *message.StickerMessage.IsAnimated {
			mediaType = "animated_sticker"
		}
		media = whatsmeow.DownloadableMessage(message.StickerMessage)
	}

	return dto.ParsedMsg{
		Body:          body,
		Media:         &media,
		MediaType:     dto.MediaType(mediaType),
		MediaFilename: mediaFilename,
	}
}
