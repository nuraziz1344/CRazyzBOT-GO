package dto

import (
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

type MediaType string

const (
	MediaImage           MediaType = "image"
	MediaVideo           MediaType = "video"
	MediaDocument        MediaType = "document"
	MediaSticker         MediaType = "sticker"
	MediaAnimatedSticker MediaType = "animated_sticker"
)

type ParsedMsg struct {
	StanzaID types.MessageID
	Message  *waE2E.Message

	QuotedMessage     *waE2E.Message
	QuotedStanzaID    *types.MessageID
	QuotedParticipant *string

	From        types.JID
	Sender      types.JID
	Participant string

	IsGroup   bool
	GroupInfo *types.GroupInfo

	PushName string
	Phone    string

	Timestamp time.Time
	Body      string

	Media         *whatsmeow.DownloadableMessage
	MediaType     MediaType
	MediaFilename string
}

type ParseQuotedMessage struct {
	Body      string
	Media     whatsmeow.DownloadableMessage
	MediaType MediaType
}

type Quoted struct {
	QuotedMessage *waE2E.Message
	StanzaID      *types.MessageID
	Participant   *string
}
