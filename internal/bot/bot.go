package bot

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nuraziz1344/CRazyzBOT-GO/internal/alert/gempa"
	"github.com/nuraziz1344/CRazyzBOT-GO/internal/helper"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var client *whatsmeow.Client

func eventHandler(evt any) {
	switch v := evt.(type) {
	case *events.Message:
		Handle(client, v)
	case *events.Connected:
		log.Println("BOT Connected!")
	}
}

func Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "warn"
	}

	log.Println("Starting BOT...")
	dbLog := waLog.Stdout("Database", logLevel, true)

	// Create a new SQLite store
	sessionFile := os.Getenv("SESSION_FILE")
	if sessionFile == "" {
		sessionFile = "data/session.db"
	}
	db, err := sqlstore.New(ctx, "sqlite3", "file:"+sessionFile+"?_foreign_keys=on", dbLog)
	if err != nil {
		return err
	}
	deviceStore, err := db.GetFirstDevice(ctx)
	if err != nil {
		return err
	}

	clientLog := waLog.Stdout("Client", logLevel, true)
	client = whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventHandler)

	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(ctx)
		err = client.Connect()
		if err != nil {
			return err
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				// Render the QR code here
				// e.g. qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				// or just manually `echo 2@... | qrencode -t ansiutf8` in a terminal
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				log.Println("QR code:", evt.Code)
			} else {
				log.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			return err
		}
	}

	startGempaAlerts(ctx, sessionFile)

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-c
	cancel()

	client.Disconnect()
	return nil
}

func startGempaAlerts(ctx context.Context, sessionFile string) {
	groupID := os.Getenv("GEMPA_ALERT_JID")
	if groupID == "" {
		log.Println("GEMPA_ALERT_JID not set; skipping gempa alerts")
		return
	}

	if strings.Contains(groupID, "@") {
		groupID = strings.SplitN(groupID, "@", 2)[0]
	}

	store, err := gempa.NewStore("file:" + sessionFile + "?_foreign_keys=on")
	if err != nil {
		log.Printf("gempa alert store error: %v\n", err)
		return
	}

	interval := time.Duration(0)
	if v := os.Getenv("GEMPA_ALERT_INTERVAL"); v != "" {
		if dur, err := time.ParseDuration(v); err == nil {
			interval = dur
		}
	}

	endpoint := os.Getenv("GEMPA_ALERT_ENDPOINT")
	fetcher := gempa.NewFetcher(store, gempa.Config{
		Endpoint: endpoint,
		Interval: interval,
	})

	alertJID := types.JID{User: groupID, Server: "g.us"}
	go func() {
		if err := fetcher.Run(ctx, func(alert gempa.Alert) {
			if len(alert.Shakemap) > 0 {
				if err := helper.SendImageMessage(client, alertJID, &alert.Shakemap, nil); err != nil {
					log.Printf("send shakemap error: %v\n", err)
				}
			} else if alert.ShakemapURL != "" {
				fallback := alert.Text + "\nShakemap: " + alert.ShakemapURL
				helper.SendTextMessage(client, alertJID, fallback, nil)
				return
			}
			helper.SendTextMessage(client, alertJID, alert.Text, nil)
		}); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("gempa alert stopped: %v\n", err)
		}
	}()
}
