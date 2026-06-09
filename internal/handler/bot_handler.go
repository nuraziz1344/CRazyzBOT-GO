package handler

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"crazyzbot-go/internal/commands"
	"crazyzbot-go/internal/config"
	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/logutil"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type BotHandler struct {
	client   *whatsmeow.Client
	registry *commands.Registry
}

func NewBotHandler(client *whatsmeow.Client, _ *config.Config, registry *commands.Registry) *BotHandler {
	return &BotHandler{
		client:   client,
		registry: registry,
	}
}

func (h *BotHandler) EventHandler(evt any) {
	switch v := evt.(type) {
	case *events.Message:
		h.handleMessage(v)
	case *events.Connected:
		slog.Info("BOT Connected!")
	}
}

func (h *BotHandler) handleMessage(msg *events.Message) {
	parsedMsg, ok := parseMessage(h.client, msg)
	if !ok {
		return
	}

	// Inject logID and request-scoped logger into context for traceability
	ctx := logutil.WithLogID(context.Background())

	logutil.Info(ctx, "Incoming message",
		"msgID", parsedMsg.StanzaID,
		"from", parsedMsg.From,
		"sender", parsedMsg.Phone,
		"pushName", parsedMsg.PushName,
	)

	h.client.MarkRead(context.Background(), []types.MessageID{types.MessageID(msg.Info.ID)}, time.Now(), msg.Info.Chat, msg.Info.Sender)
	h.registry.Handle(ctx, h.client, parsedMsg)
}

func parseMessage(client *whatsmeow.Client, msg *events.Message) (*dto.ParsedMsg, bool) {
	if time.Since(msg.Info.Timestamp).Seconds() > 60 {
		return nil, false
	}

	sender := helper.GetSenderPhone(msg.Info)
	pushName := msg.Info.PushName
	message := msg.Message
	var body string
	var groupInfo *types.GroupInfo
	var quotedMessage *waE2E.Message
	var quotedStanzaID *types.MessageID
	var quotedParticipant *string
	var media whatsmeow.DownloadableMessage
	var mediaType string
	var mediaFilename string

	if strings.Contains(msg.Info.Chat.String(), "@g.us") {
		info, err := client.GetGroupInfo(context.Background(), msg.Info.Chat)
		if err != nil {
			slog.Warn("Error getting group info", "error", err, "chat", msg.Info.Chat.String())
			return nil, false
		}
		groupInfo = info
	}

	if message.ViewOnceMessage != nil {
		message = message.ViewOnceMessage.Message
	} else if message.DocumentWithCaptionMessage != nil {
		message = message.DocumentWithCaptionMessage.Message
	}

	switch {
	case message.Conversation != nil:
		body = message.GetConversation()
	case message.ExtendedTextMessage != nil:
		body = message.ExtendedTextMessage.GetText()
		quotedMessage, quotedStanzaID, quotedParticipant = extractQuotedContext(message)
	case message.ImageMessage != nil:
		mediaType = "image"
		body = message.ImageMessage.GetCaption()
		media = message.ImageMessage
	case message.VideoMessage != nil:
		mediaType = "video"
		body = message.VideoMessage.GetCaption()
		media = message.VideoMessage
	case message.DocumentMessage != nil:
		mediaType = "document"
		media = message.DocumentMessage
		mediaFilename = message.DocumentMessage.GetFileName()
		if mediaFilename == "" {
			mediaFilename = message.DocumentMessage.GetTitle()
		}
		if message.DocumentMessage.GetTitle() != message.DocumentMessage.GetCaption() {
			body = message.DocumentMessage.GetCaption()
		}
	case message.StickerMessage != nil:
		mediaType = "sticker"
		if message.StickerMessage.IsAnimated != nil && *message.StickerMessage.IsAnimated {
			mediaType = "animated_sticker"
		}
		media = message.StickerMessage
	}

	parsedMsg := &dto.ParsedMsg{
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

		PushName: pushName,
		Phone:    sender,

		Timestamp:     msg.Info.Timestamp,
		Body:          helper.TrimString(body),
		Media:         &media,
		MediaType:     dto.MediaType(mediaType),
		MediaFilename: mediaFilename,
	}

	return parsedMsg, true
}

func extractQuotedContext(message *waE2E.Message) (*waE2E.Message, *types.MessageID, *string) {
	if message == nil || message.ExtendedTextMessage == nil {
		return nil, nil, nil
	}

	contextInfo := message.ExtendedTextMessage.GetContextInfo()
	if contextInfo == nil || contextInfo.GetQuotedMessage() == nil {
		return nil, nil, nil
	}

	return contextInfo.GetQuotedMessage(), contextInfo.StanzaID, contextInfo.Participant
}
