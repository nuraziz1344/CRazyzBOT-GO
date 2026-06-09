package commands

import (
	"context"
	"fmt"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/logutil"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
)

func HandleTagAll(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore) {
	logutil.Info(ctx, "TagAll command", "group", msg.From.String(), "sender", msg.Phone)
	if !msg.IsGroup {
		helper.SendTextMessage(ctx, c, msg.From, "This command can only be used in groups.", nil)
		return
	}

	var mentionedJIDs []string
	if msg.GroupInfo != nil {
		for _, participant := range msg.GroupInfo.Participants {
			mentionedJIDs = append(mentionedJIDs, participant.JID.String())
		}
	}

	var message string
	if args != "" {
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
	c.SendMessage(ctx, msg.From, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:        &message,
			ContextInfo: contextInfo,
		},
	})
}
