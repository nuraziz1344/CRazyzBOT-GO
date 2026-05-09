package commands

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"

	"go.mau.fi/whatsmeow"
)

const maxStickerSize = 1024 * 1024 // 1MB

func HandleSticker(c *whatsmeow.Client, msg *dto.ParsedMsg, packName string) {
	media, mediaType, ok := resolveStickerMedia(msg)
	if !ok {
		log.Println("No media found for sticker generation")
		return
	}

	res, err := c.Download(context.Background(), *media)
	if err != nil {
		log.Println("Error downloading media:", err)
		return
	}

	ext, isAnimated := detectStickerInput(mediaType, res)
	inputPath, err := writeStickerInput(res, ext)
	if err != nil {
		log.Println("Error preparing sticker input:", err)
		return
	}
	defer os.Remove(inputPath)

	res, err = runStickerFFmpeg(inputPath, isAnimated, packName, false)
	if err != nil {
		log.Println("Error generating sticker:", err)
		return
	}

	// Check if sticker is too large and suggest sticker2 command
	if len(res) > maxStickerSize {
		sizeMB := float64(len(res)) / (1024 * 1024)
		helpMsg := fmt.Sprintf("Sticker too large (%.2f MB). Try using /sticker2 for better compression.", sizeMB)
		helper.SendTextMessage(c, msg.From, helpMsg, &dto.Quoted{
			QuotedMessage: msg.QuotedMessage,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
		return
	}

	err = helper.SendStickerMessage(c, msg.From, &res, isAnimated, buildQuotedMessage(msg))
	if err != nil {
		log.Println("Error sending sticker message:", err)
		return
	}
}

func HandleSticker2(c *whatsmeow.Client, msg *dto.ParsedMsg, packName string) {
	media, mediaType, ok := resolveStickerMedia(msg)
	if !ok {
		log.Println("No media found for sticker generation")
		return
	}

	res, err := c.Download(context.Background(), *media)
	if err != nil {
		log.Println("Error downloading media:", err)
		return
	}

	ext, isAnimated := detectStickerInput(mediaType, res)
	inputPath, err := writeStickerInput(res, ext)
	if err != nil {
		log.Println("Error preparing sticker input:", err)
		return
	}
	defer os.Remove(inputPath)

	res, err = runStickerFFmpeg(inputPath, isAnimated, packName, true)
	if err != nil {
		log.Println("Error generating sticker with bitrate:", err)
		return
	}

	err = helper.SendStickerMessage(c, msg.From, &res, isAnimated, buildQuotedMessage(msg))
	if err != nil {
		log.Println("Error sending sticker message:", err)
		return
	}
}

func writeStickerInput(media []byte, ext string) (string, error) {
	inputPath := helper.Temp(ext)
	if err := os.WriteFile(inputPath, media, 0644); err != nil {
		_ = os.Remove(inputPath)
		return "", err
	}
	return inputPath, nil
}

func resolveStickerMedia(msg *dto.ParsedMsg) (*whatsmeow.DownloadableMessage, dto.MediaType, bool) {
	if isSupportedStickerMedia(msg.MediaType) {
		return msg.Media, msg.MediaType, msg.Media != nil
	}
	if msg.QuotedMessage == nil {
		return nil, "", false
	}

	quotedMsg := helper.ParseQuotedMessage(msg.QuotedMessage)
	if !isSupportedStickerMedia(quotedMsg.MediaType) || quotedMsg.Media == nil {
		return nil, "", false
	}

	return quotedMsg.Media, quotedMsg.MediaType, true
}

func isSupportedStickerMedia(mediaType dto.MediaType) bool {
	return mediaType == dto.MediaImage || mediaType == dto.MediaVideo || mediaType == dto.MediaDocument
}

func detectStickerInput(mediaType dto.MediaType, media []byte) (string, bool) {
	switch mediaType {
	case dto.MediaVideo:
		return ".mp4", true
	case dto.MediaDocument:
		mimeType := http.DetectContentType(media)
		if mimeType == "image/gif" {
			return ".gif", true
		}
		if strings.HasPrefix(mimeType, "video/") {
			return ".mp4", true
		}
	}

	return ".png", false
}

func buildQuotedMessage(msg *dto.ParsedMsg) *dto.Quoted {
	return &dto.Quoted{
		QuotedMessage: msg.QuotedMessage,
		StanzaID:      &msg.StanzaID,
		Participant:   &msg.Participant,
	}
}

func runStickerFFmpeg(inputPath string, isAnimated bool, packName string, useBitrate bool) ([]byte, error) {
	tempOutput := helper.Temp(".webp")
	defer os.Remove(tempOutput)

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, err
	}

	command := helper.GenerateFfmpegArgs(inputPath, tempOutput, isAnimated)
	if useBitrate {
		duration := 10.0
		if isAnimated {
			if d, err := helper.GetVideoDuration(inputPath); err == nil && d > 0 {
				duration = d
				if duration > 10 {
					duration = 10
				}
			}
		}
		command = helper.GenerateFfmpegArgsWithBitrate(inputPath, tempOutput, isAnimated, duration)
	}

	if err := exec.Command(ffmpeg, command...).Run(); err != nil {
		return nil, err
	}

	res, err := os.ReadFile(tempOutput)
	if err != nil {
		return nil, err
	}

	res, err = addStickerMetadata(res, resolveStickerPackName(packName), resolveStickerAuthor())
	if err != nil {
		log.Println("Warning: failed to add sticker metadata:", err)
	}

	return res, nil
}

func resolveStickerAuthor() string {
	author := os.Getenv("STICKER_PACK_AUTHOR")
	if author == "" {
		return "CRazyzBOT"
	}
	return author
}

func resolveStickerPackName(packName string) string {
	if packName != "" {
		return packName
	}
	packName = os.Getenv("STICKER_PACK_NAME")
	if packName == "" {
		return "CRazyz Stickers"
	}
	return packName
}

// addStickerMetadata adds WhatsApp-compatible EXIF metadata to WebP sticker
// Based on: https://github.com/Nurutomo/wabot-aq/blob/542ff69e4e2b82423b5875f90157dcd4f9ffb4e3/lib/sticker.js#L136
func addStickerMetadata(webpData []byte, packName, author string) ([]byte, error) {
	// Generate random sticker pack ID (32 bytes = 64 hex chars)
	packID := make([]byte, 32)
	if _, err := rand.Read(packID); err != nil {
		return nil, err
	}

	// Build metadata JSON
	metadata := map[string]interface{}{
		"sticker-pack-id":        hex.EncodeToString(packID),
		"sticker-pack-name":      packName,
		"sticker-pack-publisher": author,
		"emojis":                 []string{""},
	}

	jsonData, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	// Build EXIF structure: TIFF header + JSON payload
	// Exact format from: https://github.com/Nurutomo/wabot-aq/blob/542ff69e4e2b82423b5875f90157dcd4f9ffb4e3/lib/sticker.js#L136
	exifAttr := []byte{0x49, 0x49, 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00, 0x01, 0x00, 0x41, 0x57, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x16, 0x00, 0x00, 0x00}

	// Write JSON length at offset 14 (bytes 15-18), little-endian uint32
	binary.LittleEndian.PutUint32(exifAttr[14:18], uint32(len(jsonData)))

	// Concatenate EXIF header + JSON data
	exif := append(exifAttr, jsonData...)

	// Use webpmux to add EXIF chunk
	webpMux, err := exec.LookPath("webpmux")
	if err != nil {
		// webpmux not available, return original data
		return webpData, nil
	}

	tempInput := helper.Temp("_input.webp")
	defer os.Remove(tempInput)
	tempOutput := helper.Temp("_output.webp")
	defer os.Remove(tempOutput)

	// Write WebP to temp file
	if err := os.WriteFile(tempInput, webpData, 0644); err != nil {
		_ = os.Remove(tempInput)
		return nil, err
	}

	// Write EXIF to temp file
	tempExif := helper.Temp(".exif")
	defer os.Remove(tempExif)

	if err := os.WriteFile(tempExif, exif, 0644); err != nil {
		_ = os.Remove(tempExif)
		return nil, err
	}

	// Use webpmux to add EXIF
	cmd := exec.Command(webpMux, "-set", "exif", tempExif, tempInput, "-o", tempOutput)
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	// Read result
	result, err := os.ReadFile(tempOutput)
	if err != nil {
		return nil, err
	}

	return result, nil
}
