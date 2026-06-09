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

	"crazyzbot-go/internal/commands"
	"crazyzbot-go/internal/config"
	"crazyzbot-go/internal/handler"
	"crazyzbot-go/internal/logutil"
	"crazyzbot-go/internal/services/downloader"
	"crazyzbot-go/internal/services/earthquake"
	"crazyzbot-go/internal/services/minecraft"
	"crazyzbot-go/internal/services/prayer"
	"crazyzbot-go/internal/services/shipping"
	"crazyzbot-go/internal/storage"
)

func main() {
	cfg := config.LoadConfig()
	logutil.InitJSONLogger(cfg.LogLevel)

	log.Println("Starting BOT...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// Log levels: ERROR, WARN, INFO, DEBUG
	clientLog := waLog.Stdout("Client", "WARN", true) 
	client := whatsmeow.NewClient(deviceStore, clientLog)

	// Initialize Services
	minecraftService := minecraft.NewService()
	prayerService := prayer.NewService()
	shippingService := shipping.NewService(os.Getenv("BINDERBYTE_API_KEY"))
	downloaderService := downloader.NewService()
	earthquakeService := earthquake.NewService()

	// Initialize Storage
	subscriptionStore, err := storage.NewSQLiteSubscriptionStore(cfg.SubscriptionDBFile)
	if err != nil {
		log.Fatalf("Failed to initialize subscription storage: %v", err)
	}
	defer subscriptionStore.Close()

	// Initialize Handlers
	gameHandler := commands.NewGameHandler(minecraftService)
	religionHandler := commands.NewReligionHandler(prayerService)
	utilityHandler := commands.NewUtilityHandler(shippingService)
	downloaderHandler := commands.NewDownloaderHandler(downloaderService)

	// Register commands
	registry := commands.NewRegistry(subscriptionStore)
	registry.Register("ping", commands.HandlePing)
	registry.Register("help", commands.HandleHelp, "h")
	registry.Register("sticker", commands.HandleSticker, "s", "stiker")
	registry.Register("sticker2", commands.HandleSticker2, "s2")
	registry.Register("toimg", commands.HandleToImg)
	registry.Register("tagall", commands.HandleTagAll, "all", "everyone")

	registry.Register("minecraft", gameHandler.HandleMinecraft, "mc")
	registry.Register("sholat", religionHandler.HandlePrayer, "jadwalsholat")
	registry.Register("cekresi", utilityHandler.HandleResi)

	registry.Register("yts", downloaderHandler.HandleYTSearch)
	registry.Register("ytdl", downloaderHandler.HandleDownloader)
	registry.Register("ytdl2", downloaderHandler.HandleDownloader)
	registry.Register("tiktok", downloaderHandler.HandleDownloader, "t")
	registry.Register("instagram", downloaderHandler.HandleDownloader, "ig", "igdl")
	registry.Register("facebook", downloaderHandler.HandleDownloader, "fb", "fbdl")
	registry.Register("twitter", downloaderHandler.HandleDownloader, "x", "twitterdl")

	registry.Register("ocr", commands.HandleOCR)
	registry.Register("prayersubscribe", commands.HandlePrayerSubscribe, "psub")
	registry.Register("prayerunsubscribe", commands.HandlePrayerUnsubscribe, "punsub")
	registry.Register("earthquakesubscribe", commands.HandleEarthquakeSubscribe, "esub")
	registry.Register("earthquakeunsubscribe", commands.HandleEarthquakeUnsubscribe, "eunsub")

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

	earthquake.StartScheduler(ctx, client, earthquakeService, subscriptionStore)
	prayer.StartScheduler(ctx, client, prayerService, subscriptionStore)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-c
	cancel()

	client.Disconnect()
}
