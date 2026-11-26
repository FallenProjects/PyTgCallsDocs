package main

//go:generate go run github.com/AshokShau/gotdbot/scripts/tools

import (
	"ashokshau/pytgdocs/config"
	"ashokshau/pytgdocs/internal/bot"
	"ashokshau/pytgdocs/internal/docs"
	"log/slog"
	"os"
	"time"

	"github.com/AshokShau/gotdbot"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	docData, err := docs.Load("./docs.json")
	if err != nil {
		slog.Error("Failed to load docs.json", "error", err)
		os.Exit(1)
	}

	clientConfig := &gotdbot.ClientOpts{
		AutoRetry: &gotdbot.AutoRetry{MaxFloodWait: 5 * time.Second, MessageNotFound: true, ChatNotFound: true},
	}

	client, err := gotdbot.NewClient(cfg.APIID, cfg.APIHash, cfg.Token, clientConfig)
	if err != nil {
		slog.Error("gotdbot.NewClient error", "error", err)
		os.Exit(1)
	}

	b := bot.New(client, docData)
	b.Register()

	if err = client.Start(); err != nil {
		slog.Error("gotdbot.Start() error", "error", err)
		os.Exit(1)
	}

	client.Idle()
}
