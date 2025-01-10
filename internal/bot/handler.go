package bot

import (
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

func Handle(c *whatsmeow.Client, msg *events.Message) {
	fmt.Println("Received a message!", msg.Message.GetConversation())

	var err error

	from := msg.Info.Chat.String()
	sender := GetSenderNumber(msg.Info.Sender.String())
	pushname := msg.Info.PushName
	timestamp := msg.Info.Timestamp

	var message *waE2E.Message = msg.Message
	var media whatsmeow.DownloadableMessage
	var mediaByte []byte
	var mediaType string
	var body string

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
		if message.DocumentMessage.GetTitle() != message.DocumentMessage.GetCaption() {
			body = message.DocumentMessage.GetCaption()
		}
	}

	if media != nil {
		mediaByte, err = c.Download(media)
		if err != nil {
			fmt.Printf("Download Media Error: %s", err.Error())
		}
	}

	fmt.Println("from", from)
	fmt.Println("sender", sender)
	fmt.Println("pushname", pushname)
	fmt.Println("timestamp", timestamp.String())
	fmt.Println("message", body)
	fmt.Println("media", len(mediaByte))
	fmt.Println("media type", mediaType)
}
