package main

import (
	"log"

	_ "github.com/joho/godotenv/autoload"
	"github.com/nuraziz1344/CRazyzBOT-GO/internal/bot"
)

func main() {
	if err := bot.Start(); err != nil {
		log.Fatalf("Failed to start bot: %v", err)
	}
}
