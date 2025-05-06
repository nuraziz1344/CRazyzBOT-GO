package commands

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/nuraziz1344/CRazyzBOT-GO/internal/dto"
	"github.com/nuraziz1344/CRazyzBOT-GO/internal/helper"
	"go.mau.fi/whatsmeow"
)

func HandleSticker(c *whatsmeow.Client, msg *dto.ParsedMsg) {
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

	res, err = c.Download(*media)
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

	res, err = generateSticker(res, isAnimated)
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

func generateSticker(media []byte, isAnimated bool) ([]byte, error) {
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
	return res, nil
}
