package commands

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/nuraziz1344/CRazyzBOT-GO/internal/dto"
	"github.com/nuraziz1344/CRazyzBOT-GO/internal/helper"
	"go.mau.fi/whatsmeow"
)

func HandleSticker(c *whatsmeow.Client, msg *dto.ParsedMsg, packName string) {
	var media *whatsmeow.DownloadableMessage
	var mediaType dto.MediaType
	var isAnimated bool

	if msg.MediaType == dto.MediaImage || msg.MediaType == dto.MediaVideo || msg.MediaType == dto.MediaDocument {
		media = msg.Media
		mediaType = msg.MediaType
	} else if msg.QuotedMessage != nil {
		quotedMsg := helper.ParseQuotedMessage(msg.QuotedMessage)
		if quotedMsg.MediaType == dto.MediaImage || quotedMsg.MediaType == dto.MediaVideo || quotedMsg.MediaType == dto.MediaDocument {
			media = quotedMsg.Media
			mediaType = quotedMsg.MediaType
		}
	}

	if media == nil {
		log.Println("No media found for sticker generation")
		return
	}

	var res []byte
	var err error

	res, err = c.Download(context.Background(), *media)
	if err != nil {
		log.Println("Error downloading media:", err)
		return
	}

	if mediaType == dto.MediaVideo {
		isAnimated = true
	} else if mediaType == dto.MediaDocument {
		mimeType := http.DetectContentType(res)
		isAnimated = strings.HasPrefix(mimeType, "video/")
	}

	res, err = generateSticker(res, isAnimated, packName)
	if err != nil {
		log.Println("Error generating sticker:", err)
		return
	}

	err = helper.SendStickerMessage(c, msg.From, &res, isAnimated, &dto.Quoted{
		QuotedMessage: msg.QuotedMessage,
		StanzaID:      &msg.StanzaID,
		Participant:   &msg.Participant,
	})
	if err != nil {
		log.Println("Error sending sticker message:", err)
		return
	}
}

func generateSticker(media []byte, isAnimated bool, packName string) ([]byte, error) {
	tempOutput := helper.Temp(".webp")
	tempInput := helper.Temp(".png")
	if isAnimated {
		tempInput = helper.Temp(".mp4")
	}

	err := os.WriteFile(tempInput, media, 0644)
	if err != nil {
		return nil, err
	}

	command := helper.GenerateFfmpegArgs(tempInput, tempOutput, isAnimated)
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(ffmpeg, command...)

	defer os.Remove(tempInput)
	err = cmd.Run()
	if err != nil {
		return nil, err
	}

	res, err := os.ReadFile(tempOutput)
	if err != nil {
		return nil, err
	}

	defer os.Remove(tempOutput)

	// Add WhatsApp sticker metadata
	author := os.Getenv("STICKER_PACK_AUTHOR")
	if author == "" {
		author = "CRazyzBOT"
	}
	if packName == "" {
		packName = os.Getenv("STICKER_PACK_NAME")
		if packName == "" {
			packName = "CRazyz Stickers"
		}
	}

	res, err = addStickerMetadata(res, packName, author)
	if err != nil {
		log.Println("Warning: failed to add sticker metadata:", err)
	}

	return res, nil
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
		"sticker-pack-id":       hex.EncodeToString(packID),
		"sticker-pack-name":      packName,
		"sticker-pack-publisher": author,
		"emojis":                 []string{""},
	}

	jsonData, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	// Build EXIF structure: TIFF header + JSON payload
	// TIFF header: II (little-endian) + 0x002A + IFD offset + IFD entry
	var exifBuf bytes.Buffer

	// TIFF header: "II" (little-endian) + magic 0x002A + IFD offset (8)
	exifBuf.Write([]byte{0x49, 0x49, 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00})

	// IFD entry count (1)
	binary.Write(&exifBuf, binary.LittleEndian, uint16(1))

	// IFD entry: tag=0x5741 ("AW"), type=7 (undefined), count, value offset
	// Tag: 0x5741 ("AW" ASCII)
	binary.Write(&exifBuf, binary.LittleEndian, uint16(0x5741))
	// Type: 7 (UNDEFINED)
	binary.Write(&exifBuf, binary.LittleEndian, uint16(7))
	// Count: JSON data length
	binary.Write(&exifBuf, binary.LittleEndian, uint32(len(jsonData)))
	// Value offset: 22 (header 8 + count 2 + entry 12)
	binary.Write(&exifBuf, binary.LittleEndian, uint32(22))

	// Next IFD offset (0 = no more)
	binary.Write(&exifBuf, binary.LittleEndian, uint32(0))

	// Write JSON data
	exifBuf.Write(jsonData)

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
		return nil, err
	}

	// Write EXIF to temp file
	tempExif := helper.Temp(".exif")
	defer os.Remove(tempExif)

	if err := os.WriteFile(tempExif, exifBuf.Bytes(), 0644); err != nil {
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
