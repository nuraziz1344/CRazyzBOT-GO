package bot

import (
	"log"
	"strings"
	"time"

	"github.com/nuraziz1344/CRazyzBOT-GO/internal/bot/commands"
	"github.com/nuraziz1344/CRazyzBOT-GO/internal/dto"
	"github.com/nuraziz1344/CRazyzBOT-GO/internal/helper"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func Handle(c *whatsmeow.Client, msg *events.Message) {
	// log.Println("Received a message!", msg.Message.GetConversation())
	var err error

	sender := helper.GetSenderNumber(msg.Info.Sender.String())
	pushname := msg.Info.PushName
	timestamp := msg.Info.Timestamp

	// Ignore messages older than 60 seconds
	if time.Since(timestamp).Seconds() > 60 {
		return
	}

	var groupInfo *types.GroupInfo
	var message *waE2E.Message = msg.Message
	var body string

	var quotedMessage *waE2E.Message
	var quotedStanzaID *types.MessageID
	var quotedParticipant *string

	var media whatsmeow.DownloadableMessage
	var mediaType string
	var mediaFilename string

	if strings.Contains(msg.Info.Chat.String(), "@g.us") {
		groupInfo, err = c.GetGroupInfo(msg.Info.Chat)
		if err != nil {
			log.Println("Error getting group info:", err)
			return
		}
	}

	if message.ViewOnceMessage != nil {
		message = message.ViewOnceMessage.Message
	} else if message.DocumentWithCaptionMessage != nil {
		message = message.DocumentWithCaptionMessage.Message
	}

	if message.Conversation != nil {
		body = message.GetConversation()
	} else if message.ExtendedTextMessage != nil {
		body = message.ExtendedTextMessage.GetText()
		if message.ExtendedTextMessage.ContextInfo.QuotedMessage != nil {
			quotedMessage = message.ExtendedTextMessage.ContextInfo.QuotedMessage
			quotedStanzaID = message.ExtendedTextMessage.ContextInfo.StanzaID
			quotedParticipant = message.ExtendedTextMessage.ContextInfo.Participant
		}
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
		if message.DocumentMessage.GetFileName() == "" {
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
		media = message.StickerMessage
	}

	parsedMsg := dto.ParsedMsg{
		StanzaID: msg.Info.ID,
		Message:  message,

		QuotedMessage:     quotedMessage,
		QuotedStanzaID:    quotedStanzaID,
		QuotedParticipant: quotedParticipant,

		From:        msg.Info.Chat,
		Sender:      msg.Info.Sender,
		Participant: sender + "@s.whatsapp.net",

		IsGroup:   strings.Contains(msg.Info.Chat.String(), "@g.us"),
		GroupInfo: groupInfo,

		PushName: pushname,
		Phone:    sender,

		Timestamp:     timestamp,
		Body:          helper.TrimString(body),
		Media:         &media,
		MediaType:     dto.MediaType(mediaType),
		MediaFilename: mediaFilename,
	}

	// helper.PrettyPrint(parsedMsg)
	commands.HandleCommand(c, &parsedMsg)
	c.MarkRead([]string{msg.Info.ID}, time.Now(), msg.Info.Chat, msg.Info.Sender)
}

func GetGroupName(c *whatsmeow.Client, JID types.JID) string {
	groups, err := c.GetGroupInfo(JID)
	if err != nil {
		log.Println("Error getting group name:", err)
		return ""
	}
	return groups.Name
}
