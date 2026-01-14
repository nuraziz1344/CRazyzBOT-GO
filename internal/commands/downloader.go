package commands

import (
	"context"
	"fmt"
	"io"
	"log"

	"bot/internal/dto"
	"bot/internal/helper"
	"bot/internal/services/downloader"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
)

type DownloaderHandler struct {
	service *downloader.Service
}

func NewDownloaderHandler(service *downloader.Service) *DownloaderHandler {
	return &DownloaderHandler{
		service: service,
	}
}

func (h *DownloaderHandler) HandleYTSearch(c *whatsmeow.Client, msg *dto.ParsedMsg, args string) {
	if args == "" {
		helper.SendTextMessage(c, msg.From, "Usage: /yts <query>", nil)
		return
	}

	res, err := h.service.Search(context.Background(), args, 5)
	if err != nil {
		helper.SendTextMessage(c, msg.From, "Error searching: "+err.Error(), nil)
		return
	}

	if len(res) == 0 {
		helper.SendTextMessage(c, msg.From, "No results found", nil)
		return
	}

	reply := "YouTube Search Results:\n"
	for i, video := range res {
		reply += fmt.Sprintf("%d. %s (%s)\n%s\n\n", i+1, video.Title, video.Uploader, video.Webpage)
	}
	reply += "Type /ytdl <url> to download."

	helper.SendTextMessage(c, msg.From, reply, nil)
}

func (h *DownloaderHandler) HandleDownloader(c *whatsmeow.Client, msg *dto.ParsedMsg, args string) {
	url := args
	if url == "" {
		helper.SendTextMessage(c, msg.From, "Please provide a URL", nil)
		return
	}

	helper.SendTextMessage(c, msg.From, "Processing...", nil)

	ctx := context.Background()

	// Get metadata
	meta, err := h.service.GetVideoMetadata(ctx, url)
	if err != nil {
		helper.SendTextMessage(c, msg.From, "Failed to get metadata: "+err.Error(), nil)
		return
	}

	// Prepare stream
	cmd, err := h.service.GetStream(ctx, url, false)
	if err != nil {
		helper.SendTextMessage(c, msg.From, "Failed to prepare download", nil)
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		helper.SendTextMessage(c, msg.From, "Failed to pipe output", nil)
		return
	}

	if err := cmd.Start(); err != nil {
		helper.SendTextMessage(c, msg.From, "Failed to start download", nil)
		return
	}
	defer cmd.Wait()

	data, err := io.ReadAll(stdout)
	if err != nil {
		helper.SendTextMessage(c, msg.From, "Failed to read stream", nil)
		return
	}

	uploaded, err := c.Upload(ctx, data, whatsmeow.MediaVideo)
	if err != nil {
		helper.SendTextMessage(c, msg.From, "Failed to upload media", nil)
		log.Println("Upload error:", err)
		return
	}

	c.SendMessage(ctx, msg.From, &waE2E.Message{
		VideoMessage: &waE2E.VideoMessage{
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    &uploaded.FileLength,
			Mimetype:      protoPtr("video/mp4"),
			Caption:       &meta.Title,
		},
	})
}

func protoPtr(s string) *string {
	return &s
}
