package commands

import (
	"context"
	"fmt"

	"github.com/nuraziz1344/CRazyzBOT-GO/internal/dto"
	"github.com/nuraziz1344/CRazyzBOT-GO/internal/helper"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
)

func HandleTagAll(c *whatsmeow.Client, msg *dto.ParsedMsg, args string) {
	var mentionedJIDs []string
	for _, participant := range msg.GroupInfo.Participants {
		mentionedJIDs = append(mentionedJIDs, participant.JID.String())
	}

	var message string
	if args == "@all" || args == "@everyone" || args != "" {
		message = args
	} else {
		message = "Tagging all members in the group"
		for _, jid := range mentionedJIDs {
			message += fmt.Sprintf("\n@%s", helper.GetSenderNumber(jid))
		}
	}

	var contextInfo *waE2E.ContextInfo
	if msg.QuotedMessage != nil {
		contextInfo = helper.GenerateReplyContextInfo(&dto.Quoted{
			QuotedMessage: msg.QuotedMessage,
			StanzaID:      msg.QuotedStanzaID,
			Participant:   msg.QuotedParticipant,
		})
	} else {
		contextInfo = helper.GenerateReplyContextInfo(&dto.Quoted{
			QuotedMessage: msg.Message,
			StanzaID:      &msg.StanzaID,
			Participant:   &msg.Participant,
		})
	}

	contextInfo.MentionedJID = mentionedJIDs
	c.SendMessage(context.Background(), msg.From, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:        &message,
			ContextInfo: contextInfo,
		},
	})
}
