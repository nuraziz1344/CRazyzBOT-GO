package bot

import (
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

func Handle(c *whatsmeow.Client, msg *events.Message) {
	fmt.Println("Received a message!", msg.Message.GetConversation())
}
