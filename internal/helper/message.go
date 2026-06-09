package helper

import (
	"context"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/logutil"

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

func SendTextMessage(ctx context.Context, c *whatsmeow.Client, JID types.JID, text string, quoted *dto.Quoted) {
	var err error
	logger := logutil.LoggerFromContext(ctx)

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
		logger.Error("Error sending message", "error", err, "to", JID.String())
	}
}

func SendStickerMessage(ctx context.Context, c *whatsmeow.Client, from types.JID, media *[]byte, isAnimated bool, quoted *dto.Quoted) error {
	var err error
	logger := logutil.LoggerFromContext(ctx)

	res, err := c.Upload(context.Background(), *media, whatsmeow.MediaImage)
	if err != nil {
		return err
	}

	stickerMessage := &waE2E.StickerMessage{
		URL:           &res.URL,
		FileSHA256:    res.FileSHA256,
		FileEncSHA256: res.FileEncSHA256,
		MediaKey:      res.MediaKey,
		Mimetype:      proto.String("image/webp"),
		DirectPath:    &res.DirectPath,
		FileLength:    &res.FileLength,
		IsAnimated:    &isAnimated,
	}

	if quoted != nil {
		stickerMessage.ContextInfo = GenerateReplyContextInfo(quoted)
	}
	_, err = c.SendMessage(context.Background(), from, &waE2E.Message{StickerMessage: stickerMessage})

	if err != nil {
		logger.Error("Error sending sticker message", "error", err, "to", from.String())
	}

	return nil
}

func SendImageMessage(ctx context.Context, c *whatsmeow.Client, from types.JID, media *[]byte, quoted *dto.Quoted) error {
	var err error
	logger := logutil.LoggerFromContext(ctx)

	res, err := c.Upload(context.Background(), *media, whatsmeow.MediaImage)
	if err != nil {
		return err
	}

	imageMessage := &waE2E.ImageMessage{
		URL:           &res.URL,
		FileSHA256:    res.FileSHA256,
		FileEncSHA256: res.FileEncSHA256,
		MediaKey:      res.MediaKey,
		Mimetype:      proto.String("image/png"),
		DirectPath:    &res.DirectPath,
		FileLength:    &res.FileLength,
	}

	if quoted != nil {
		imageMessage.ContextInfo = GenerateReplyContextInfo(quoted)
	}
	_, err = c.SendMessage(context.Background(), from, &waE2E.Message{ImageMessage: imageMessage})

	if err != nil {
		logger.Error("Error sending image message", "error", err, "to", from.String())
	}

	return nil
}

func SendImageMessageWithCaption(ctx context.Context, c *whatsmeow.Client, from types.JID, media *[]byte, caption string, quoted *dto.Quoted) error {
	var err error
	logger := logutil.LoggerFromContext(ctx)

	res, err := c.Upload(context.Background(), *media, whatsmeow.MediaImage)
	if err != nil {
		return err
	}

	imageMessage := &waE2E.ImageMessage{
		URL:           &res.URL,
		FileSHA256:    res.FileSHA256,
		FileEncSHA256: res.FileEncSHA256,
		MediaKey:      res.MediaKey,
		Mimetype:      proto.String("image/png"),
		DirectPath:    &res.DirectPath,
		FileLength:    &res.FileLength,
	}

	if caption != "" {
		imageMessage.Caption = proto.String(caption)
	}

	if quoted != nil {
		imageMessage.ContextInfo = GenerateReplyContextInfo(quoted)
	}
	_, err = c.SendMessage(context.Background(), from, &waE2E.Message{ImageMessage: imageMessage})

	if err != nil {
		logger.Error("Error sending image with caption", "error", err, "to", from.String())
	}

	return nil
}

func SendGifMessage(ctx context.Context, c *whatsmeow.Client, from types.JID, media *[]byte, quoted *dto.Quoted) error {
	var err error
	logger := logutil.LoggerFromContext(ctx)

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
		logger.Error("Error sending GIF message", "error", err, "to", from.String())
	}

	return nil
}

func SendVideoMessage(ctx context.Context, c *whatsmeow.Client, from types.JID, media *[]byte, caption string, quoted *dto.Quoted) error {
	logger := logutil.LoggerFromContext(ctx)

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
	}

	if caption != "" {
		videoMessage.Caption = proto.String(caption)
	}

	if quoted != nil {
		videoMessage.ContextInfo = GenerateReplyContextInfo(quoted)
	}

	_, err = c.SendMessage(context.Background(), from, &waE2E.Message{VideoMessage: videoMessage})
	if err != nil {
		logger.Error("Error sending video message", "error", err, "to", from.String())
	}

	return err
}

func SendAudioMessage(ctx context.Context, c *whatsmeow.Client, from types.JID, media *[]byte, mimeType string, filename string, quoted *dto.Quoted) error {
	logger := logutil.LoggerFromContext(ctx)

	if mimeType == "" {
		mimeType = "audio/mpeg"
	}

	res, err := c.Upload(context.Background(), *media, whatsmeow.MediaAudio)
	if err != nil {
		return err
	}

	audioMessage := &waE2E.AudioMessage{
		URL:           &res.URL,
		Mimetype:      proto.String(mimeType),
		FileSHA256:    res.FileSHA256,
		FileEncSHA256: res.FileEncSHA256,
		FileLength:    &res.FileLength,
		MediaKey:      res.MediaKey,
		DirectPath:    &res.DirectPath,
		PTT:           proto.Bool(false),
	}

	if quoted != nil {
		audioMessage.ContextInfo = GenerateReplyContextInfo(quoted)
	}

	_, err = c.SendMessage(context.Background(), from, &waE2E.Message{AudioMessage: audioMessage})
	if err != nil {
		logger.Error("Error sending audio message", "error", err, "to", from.String())
	}

	return err
}

func SendDocumentMessage(ctx context.Context, c *whatsmeow.Client, from types.JID, media *[]byte, mimeType string, filename string, caption string, quoted *dto.Quoted) error {
	logger := logutil.LoggerFromContext(ctx)

	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	if filename == "" {
		filename = "download"
	}

	res, err := c.Upload(context.Background(), *media, whatsmeow.MediaDocument)
	if err != nil {
		return err
	}

	documentMessage := &waE2E.DocumentMessage{
		URL:           &res.URL,
		Mimetype:      proto.String(mimeType),
		Title:         proto.String(filename),
		FileName:      proto.String(filename),
		FileSHA256:    res.FileSHA256,
		FileEncSHA256: res.FileEncSHA256,
		FileLength:    &res.FileLength,
		MediaKey:      res.MediaKey,
		DirectPath:    &res.DirectPath,
	}

	if caption != "" {
		documentMessage.Caption = proto.String(caption)
	}

	if quoted != nil {
		documentMessage.ContextInfo = GenerateReplyContextInfo(quoted)
	}

	_, err = c.SendMessage(context.Background(), from, &waE2E.Message{DocumentMessage: documentMessage})
	if err != nil {
		logger.Error("Error sending document message", "error", err, "to", from.String())
	}

	return err
}
