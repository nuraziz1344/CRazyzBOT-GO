package commands

import (
	"context"
	"os"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/services/ocr"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
)

func HandleOCR(c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
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
		helper.SendTextMessage(c, msg.From, "Please send/reply to an image with /ocr", nil)
		return
	}

	data, err := c.Download(context.Background(), *media)
	if err != nil {
		helper.SendTextMessage(c, msg.From, "Failed to download image", nil)
		return
	}

	// Save to temp
	tempFile := helper.Temp(".jpg")
	err = os.WriteFile(tempFile, data, 0644)
	if err != nil {
		helper.SendTextMessage(c, msg.From, "Failed to save temp file", nil)
		return
	}
	defer os.Remove(tempFile)

	text, err := ocr.Recognize(tempFile)
	if err != nil {
		helper.SendTextMessage(c, msg.From, "OCR Failed: "+err.Error(), nil)
		return
	}

	if text == "" {
		text = "No text detected."
	}

	helper.SendTextMessage(c, msg.From, text, nil)
}
