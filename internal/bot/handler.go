package bot

import (
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

func Handle(c *whatsmeow.Client, evt interface{}) {

	switch v := evt.(type) {
	case *events.Message:
		fmt.Println("Received a message!", v.Message.GetConversation())
	case *events.Connected:
		fmt.Println("BOT Connected!")
	}

}
