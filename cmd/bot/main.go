package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/joho/godotenv/autoload"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"

	"bot/internal/commands"
	"bot/internal/config"
	"bot/internal/handler"
	"bot/internal/services/downloader"
	"bot/internal/services/minecraft"
	"bot/internal/services/prayer"
	"bot/internal/services/shipping"
)

func main() {
	cfg := config.LoadConfig()

	log.Println("Starting BOT...")
	dbLog := waLog.Stdout("Database", cfg.LogLevel, true)

	// Create a new SQLite store
	db, err := sqlstore.New(context.Background(), "sqlite3", "file:"+cfg.SessionFile+"?_foreign_keys=on", dbLog)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	deviceStore, err := db.GetFirstDevice(context.Background())
	if err != nil {
		log.Fatalf("Failed to get device store: %v", err)
	}

	clientLog := waLog.Stdout("Client", cfg.LogLevel, true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	// Initialize Services
	minecraftService := minecraft.NewService()
	prayerService := prayer.NewService()
	shippingService := shipping.NewService(os.Getenv("BINDERBYTE_API_KEY"))
	downloaderService := downloader.NewService()

	// Initialize Handlers
	gameHandler := commands.NewGameHandler(minecraftService)
	religionHandler := commands.NewReligionHandler(prayerService)
	utilityHandler := commands.NewUtilityHandler(shippingService)
	downloaderHandler := commands.NewDownloaderHandler(downloaderService)

	// Register commands
	registry := commands.NewRegistry()
	registry.Register("ping", commands.HandlePing)
	registry.Register("help", commands.HandleHelp, "h")
	registry.Register("sticker", commands.HandleSticker, "s", "stiker")
	registry.Register("toimg", commands.HandleToImg)
	registry.Register("tagall", commands.HandleTagAll, "all", "everyone")

	registry.Register("minecraft", gameHandler.HandleMinecraft, "mc")
	registry.Register("sholat", religionHandler.HandlePrayer, "jadwalsholat")
	registry.Register("cekresi", utilityHandler.HandleResi)

	registry.Register("yts", downloaderHandler.HandleYTSearch)
	registry.Register("ytdl", downloaderHandler.HandleDownloader)
	registry.Register("tiktok", downloaderHandler.HandleDownloader, "tt")
	registry.Register("instagram", downloaderHandler.HandleDownloader, "ig", "igdl")
	registry.Register("facebook", downloaderHandler.HandleDownloader, "fb", "fbdl")
	registry.Register("twitter", downloaderHandler.HandleDownloader, "x", "twitterdl")

	registry.Register("ocr", commands.HandleOCR)

	// Initialize Handler
	botHandler := handler.NewBotHandler(client, cfg, registry)
	client.AddEventHandler(botHandler.EventHandler)

	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			log.Fatalf("Failed to connect: %v", err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				log.Println("QR code:", evt.Code)
			} else {
				log.Println("Login event:", evt.Event)
			}
		}
	} else {
		err = client.Connect()
		if err != nil {
			log.Fatalf("Failed to connect: %v", err)
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-c

	client.Disconnect()
}
