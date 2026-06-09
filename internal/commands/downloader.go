package commands

import (
	"context"
	"fmt"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/logutil"
	"crazyzbot-go/internal/services"
	"crazyzbot-go/internal/services/downloader"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
)

type DownloaderHandler struct {
	service *downloader.Service
}

func NewDownloaderHandler(service *downloader.Service) *DownloaderHandler {
	return &DownloaderHandler{
		service: service,
	}
}

func (h *DownloaderHandler) HandleYTSearch(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	if args == "" {
		helper.SendTextMessage(ctx, c, msg.From, "Usage: /yts <query>", nil)
		return
	}

	res, err := h.service.Search(ctx, args, 5)
	if err != nil {
		helper.SendTextMessage(ctx, c, msg.From, "Error searching: "+err.Error(), nil)
		return
	}

	if len(res) == 0 {
		helper.SendTextMessage(ctx, c, msg.From, "No results found", nil)
		return
	}

	reply := "YouTube Search Results:\n"
	for i, video := range res {
		reply += fmt.Sprintf("%d. %s (%s)\n%s\n\n", i+1, video.Title, video.Uploader, video.Webpage)
	}
	reply += "Type /ytdl <url> to download."

	helper.SendTextMessage(ctx, c, msg.From, reply, nil)
}

func (h *DownloaderHandler) HandleDownloader(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	url := args
	if url == "" {
		helper.SendTextMessage(ctx, c, msg.From, "Please provide a URL", nil)
		return
	}

	helper.SendTextMessage(ctx, c, msg.From, "Processing...", nil)

	results, err := h.service.Resolve(ctx, url)
	if err != nil {
		helper.SendTextMessage(ctx, c, msg.From, "Failed to download: "+err.Error(), nil)
		return
	}
	if len(results) == 0 {
		helper.SendTextMessage(ctx, c, msg.From, "No downloadable media found", nil)
		return
	}

	for i, result := range results {
		if err := sendDownloadResult(ctx, c, msg, result, i == 0); err != nil {
			logutil.Error(ctx, "Downloader send error", "error", err)
			helper.SendTextMessage(ctx, c, msg.From, "Failed to send media", nil)
			return
		}
	}
}

func sendDownloadResult(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, result *services.DownloadResult, includeCaption bool) error {
	caption := ""
	if includeCaption {
		caption = result.Caption
		if caption == "" {
			caption = result.Title
		}
	}

	switch result.Type {
	case "image":
		if caption != "" {
			return helper.SendImageMessageWithCaption(ctx, c, msg.From, &result.Buffer, caption, nil)
		}
		return helper.SendImageMessage(ctx, c, msg.From, &result.Buffer, nil)
	case "video":
		return helper.SendVideoMessage(ctx, c, msg.From, &result.Buffer, caption, nil)
	case "audio":
		return helper.SendAudioMessage(ctx, c, msg.From, &result.Buffer, result.MimeType, result.Filename, nil)
	case "document":
		return helper.SendDocumentMessage(ctx, c, msg.From, &result.Buffer, result.MimeType, result.Filename, caption, nil)
	default:
		return fmt.Errorf("unsupported media type: %s", result.Type)
	}
}
