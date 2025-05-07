package helper

import (
	"context"
	"log"

	"github.com/nuraziz1344/CRazyzBOT-GO/internal/dto"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func GenerateReplyContextInfo(quoted *dto.Quoted) *waE2E.ContextInfo {
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
				ContextInfo: GenerateReplyContextInfo(quoted),
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
		stickerMessage.ContextInfo = GenerateReplyContextInfo(quoted)
	}
	_, err = c.SendMessage(context.Background(), from, &waE2E.Message{StickerMessage: stickerMessage})

	if err != nil {
		log.Println("Error sending message:", err)
	}

	return nil
}

func SendImageMessage(c *whatsmeow.Client, from types.JID, media *[]byte, quoted *dto.Quoted) error {
	var err error

	res, err := c.Upload(context.Background(), *media, whatsmeow.MediaImage)
	if err != nil {
		return err
	}

	imageMessage := &waE2E.ImageMessage{
		URL:           &res.URL,
		FileSHA256:    res.FileSHA256,
		FileEncSHA256: res.FileEncSHA256,
		MediaKey:      res.MediaKey,
		Mimetype:      StringPtr("image/png"),
		DirectPath:    &res.DirectPath,
		FileLength:    &res.FileLength,
	}

	if quoted != nil {
		imageMessage.ContextInfo = GenerateReplyContextInfo(quoted)
	}
	_, err = c.SendMessage(context.Background(), from, &waE2E.Message{ImageMessage: imageMessage})

	if err != nil {
		log.Println("Error sending message:", err)
	}

	return nil
}

func SendGifMessage(c *whatsmeow.Client, from types.JID, media *[]byte, quoted *dto.Quoted) error {
	var err error

	res, err := c.Upload(context.Background(), *media, whatsmeow.MediaVideo)
	if err != nil {
		return err
	}

	videoMessage := &waE2E.VideoMessage{
		URL:           &res.URL,
		Mimetype:      proto.String("video/mp4"),
		FileSHA256:    res.FileSHA256,
		FileEncSHA256: res.FileEncSHA256,
		FileLength:    &res.FileLength,
		MediaKey:      res.MediaKey,
		DirectPath:    &res.DirectPath,
		GifPlayback:   proto.Bool(true),
	}

	if quoted != nil {
		videoMessage.ContextInfo = GenerateReplyContextInfo(quoted)
	}
	_, err = c.SendMessage(context.Background(), from, &waE2E.Message{VideoMessage: videoMessage})

	if err != nil {
		log.Println("Error sending message:", err)
	}

	return nil
}
