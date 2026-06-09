package commands

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/logutil"
	"crazyzbot-go/internal/storage"

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

func HandleToImg(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	sticker := getSticker(msg)
	if sticker == nil {
		helper.SendTextMessage(ctx, c, msg.From, "Please send a sticker or reply to a sticker with this command.", &dto.Quoted{
			QuotedMessage: msg.Message,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
		return
	}

	bytes, err := c.Download(ctx, *sticker)
	if err != nil {
		helper.SendTextMessage(ctx, c, msg.From, "Failed to download sticker.", nil)
		logutil.Error(ctx, "Failed to download sticker", "error", err)
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
		helper.SendTextMessage(ctx, c, msg.From, "Failed to convert sticker.", nil)
		logutil.Error(ctx, "Failed to convert sticker", "error", err)
		return
	}

	if animated {
		err = helper.SendGifMessage(ctx, c, msg.From, &out, &dto.Quoted{
			QuotedMessage: msg.Message,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
	} else {
		err = helper.SendImageMessage(ctx, c, msg.From, &out, &dto.Quoted{
			QuotedMessage: msg.Message,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
	}

	if err != nil {
		logutil.Error(ctx, "Failed to send image", "error", err)
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
