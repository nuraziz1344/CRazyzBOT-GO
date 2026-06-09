package commands

import (
	"context"
	"os"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/logutil"
	"crazyzbot-go/internal/services/ocr"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
)

func HandleOCR(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	logutil.Info(ctx, "OCR command", "from", msg.From.String())
	var media *whatsmeow.DownloadableMessage

	if msg.MediaType == dto.MediaImage {
		media = msg.Media
	} else if msg.QuotedMessage != nil {
		quoted := helper.ParseQuotedMessage(msg.QuotedMessage)
		if quoted.MediaType == dto.MediaImage {
			media = quoted.Media
		}
	}

	if media == nil {
		helper.SendTextMessage(ctx, c, msg.From, "Please send/reply to an image with /ocr", nil)
		return
	}

	data, err := c.Download(ctx, *media)
	if err != nil {
		logutil.Error(ctx, "OCR download failed", "error", err)
		helper.SendTextMessage(ctx, c, msg.From, "Failed to download image", nil)
		return
	}

	// Save to temp
	tempFile := helper.Temp(".jpg")
	err = os.WriteFile(tempFile, data, 0644)
	if err != nil {
		logutil.Error(ctx, "OCR temp file write failed", "error", err)
		helper.SendTextMessage(ctx, c, msg.From, "Failed to save temp file", nil)
		return
	}
	defer os.Remove(tempFile)

	text, err := ocr.Recognize(tempFile)
	if err != nil {
		logutil.Error(ctx, "OCR recognition failed", "error", err)
		helper.SendTextMessage(ctx, c, msg.From, "OCR Failed: "+err.Error(), nil)
		return
	}

	if text == "" {
		text = "No text detected."
	}

	helper.SendTextMessage(ctx, c, msg.From, text, nil)
}
