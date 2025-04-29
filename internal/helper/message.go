package helper

import (
	"context"
	"log"

	"github.com/nuraziz1344/CRazyzBOT-GO/internal/dto"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

func generateReplyContextInfo(quoted *dto.Quoted) *waE2E.ContextInfo {
	if quoted.QuotedMessage == nil {
		return nil
	}

	return &waE2E.ContextInfo{
		QuotedMessage: quoted.QuotedMessage,
		StanzaID:      quoted.StanzaID,
		Participant:   quoted.Participant,
	}
}

func SendTextMessage(c *whatsmeow.Client, JID types.JID, text string, quoted *dto.Quoted) {
	var err error

	if quoted != nil {
		m := &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text:        &text,
				ContextInfo: generateReplyContextInfo(quoted),
			},
		}
		_, err = c.SendMessage(context.Background(), JID, m)
	} else {
		_, err = c.SendMessage(context.Background(), JID, &waE2E.Message{Conversation: &text})
	}

	if err != nil {
		log.Println("Error sending message:", err)
	}
}

func SendStickerMessage(c *whatsmeow.Client, from types.JID, media *[]byte, isAnimated bool, quoted *dto.Quoted) error {
	var err error

	res, err := c.Upload(context.Background(), *media, whatsmeow.MediaImage)
	if err != nil {
		return err
	}

	stickerMessage := &waE2E.StickerMessage{
		URL:           &res.URL,
		FileSHA256:    res.FileSHA256,
		FileEncSHA256: res.FileEncSHA256,
		MediaKey:      res.MediaKey,
		Mimetype:      StringPtr("image/webp"),
		DirectPath:    &res.DirectPath,
		FileLength:    &res.FileLength,
		IsAnimated:    &isAnimated,
	}

	if quoted != nil {
		stickerMessage.ContextInfo = generateReplyContextInfo(quoted)
	}
	_, err = c.SendMessage(context.Background(), from, &waE2E.Message{StickerMessage: stickerMessage})

	if err != nil {
		log.Println("Error sending message:", err)
	}

	return nil
}
