package commands

import (
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/nuraziz1344/CRazyzBOT-GO/internal/dto"
	"github.com/nuraziz1344/CRazyzBOT-GO/internal/helper"
	"go.mau.fi/whatsmeow"
)

func getSticker(msg *dto.ParsedMsg) (media *whatsmeow.DownloadableMessage) {
	if msg.MediaType == dto.MediaSticker || msg.MediaType == dto.MediaAnimatedSticker {
		return msg.Media
	}

	if msg.QuotedMessage != nil {
		quotedMsg := helper.ParseQuotedMessage(msg.QuotedMessage)
		if quotedMsg.MediaType == dto.MediaSticker || quotedMsg.MediaType == dto.MediaAnimatedSticker {
			return quotedMsg.Media
		}
	}
	return nil
}

func HandleToImg(c *whatsmeow.Client, msg *dto.ParsedMsg) {
	sticker := getSticker(msg)
	if sticker == nil {
		helper.SendTextMessage(c, msg.From, "Please send a sticker or reply to a sticker with this command.", &dto.Quoted{
			QuotedMessage: msg.Message,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
		return
	}

	bytes, err := c.Download(*sticker)
	if err != nil {
		helper.SendTextMessage(c, msg.From, "Failed to download sticker.", &dto.Quoted{
			QuotedMessage: msg.Message,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
		log.Printf("Failed to download sticker: %v\n", err)
		return
	}

	var out []byte
	animated := isAnimated(bytes)
	if animated {
		out, err = toMp4(bytes)
	} else {
		out, err = toImg(bytes)
	}

	if err != nil {
		helper.SendTextMessage(c, msg.From, "Failed to convert sticker.", &dto.Quoted{
			QuotedMessage: msg.Message,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
		log.Printf("Failed to convert sticker: %v\n", err)
		return
	}

	if animated {
		err = helper.SendGifMessage(c, msg.From, &out, &dto.Quoted{
			QuotedMessage: msg.Message,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
	} else {
		err = helper.SendImageMessage(c, msg.From, &out, &dto.Quoted{
			QuotedMessage: msg.Message,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
	}

	if err != nil {
		helper.SendTextMessage(c, msg.From, "Failed to send image.", &dto.Quoted{
			QuotedMessage: msg.Message,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
		log.Printf("Failed to send image: %v\n", err)
		return
	}
}

func toImg(b []byte) ([]byte, error) {
	tempInput := helper.Temp(".webp")
	tempOutput := helper.Temp(".png")

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, err
	}

	err = os.WriteFile(tempInput, b, 0644)
	if err != nil {
		return nil, err
	}

	defer os.Remove(tempInput)
	err = exec.Command(ffmpeg, "-i", tempInput, tempOutput).Run()
	if err != nil {
		return nil, err
	}

	defer os.Remove(tempOutput)
	b, err = os.ReadFile(tempOutput)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func toMp4(b []byte) ([]byte, error) {
	tempInput := helper.Temp(".webp")
	tempOutput := helper.Temp(".mp4")

	iMagick, err := exec.LookPath("magick")
	if err != nil {
		return nil, err
	}

	err = os.WriteFile(tempInput, b, 0644)
	if err != nil {
		return nil, err
	}

	defer os.Remove(tempInput)
	err = exec.Command(iMagick, "convert", tempInput, tempOutput).Run()
	if err != nil {
		return nil, err
	}

	defer os.Remove(tempOutput)
	b, err = os.ReadFile(tempOutput)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func isAnimated(b []byte) bool {
	iMagick, err := exec.LookPath("magick")
	if err != nil {
		return false
	}

	tempInput := helper.Temp(".webp")
	err = os.WriteFile(tempInput, b, 0644)
	if err != nil {
		return false
	}

	defer os.Remove(tempInput)
	out, err := exec.Command(iMagick, "identify", tempInput).Output()
	if err != nil {
		return false
	}

	outputs := strings.Split(helper.TrimString(string(out)), "\n")
	return len(outputs) > 1
}
