package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	gosalebot "gosalebot/bot"
	gosaledb "gosalebot/db"
	"gosalebot/fsm"

	bot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	_ "github.com/mattn/go-sqlite3"
)

var (
	ModerationGroupID int64
	ApprovedGroupID   int64
)

func startExpirationWorker(db *sql.DB, interval time.Duration) {
	go func() {
		for {
			rows, err := db.Query(`SELECT id, user_id, title FROM posts WHERE status = 'pending' AND expires_at < datetime('now')`)
			if err == nil {
				for rows.Next() {
					var id, userID int64
					var title string
					if err := rows.Scan(&id, &userID, &title); err == nil {
						log.Printf("Post expired: id=%d, user_id=%d, title=%s", id, userID, title)
						// Optionally, notify admin or update status
					}
				}
				rows.Close()
			}
			time.Sleep(interval)
		}
	}()
}

// No longer needed: handleUpdate. All update handling is now done via github.com/go-telegram/bot handlers.

func main() {
	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	if telegramToken == "" {
		log.Fatal("TELEGRAM_TOKEN environment variable is required")
	}

	modGroup := os.Getenv("MODERATION_GROUP_ID")
	approvedGroup := os.Getenv("APPROVED_GROUP_ID")
	moderationTopic := os.Getenv("MODERATION_TOPIC_ID")
	if modGroup == "" || approvedGroup == "" {
		log.Fatal("MODERATION_GROUP_ID and APPROVED_GROUP_ID environment variables are required")
	}
	var err error
	ModerationGroupID, err = strconv.ParseInt(modGroup, 10, 64)
	if err != nil {
		log.Fatalf("Invalid MODERATION_GROUP_ID: %v", err)
	}
	ApprovedGroupID, err = strconv.ParseInt(approvedGroup, 10, 64)
	if err != nil {
		log.Fatalf("Invalid APPROVED_GROUP_ID: %v", err)
	}
	ModerationTopicID := 0
	if moderationTopic != "" {
		ModerationTopicID, err = strconv.Atoi(moderationTopic)
		if err != nil {
			log.Fatalf("Invalid MODERATION_TOPIC_ID: %v", err)
		}
	}

	db, err := sql.Open("sqlite3", "./data/gosalebot.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	// Enable foreign key support for SQLite
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		log.Fatalf("Failed to enable foreign keys: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS posts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	chat_id INTEGER NOT NULL,
	message_id INTEGER NOT NULL,
	status TEXT NOT NULL CHECK(status IN ('pending', 'approved', 'rejected')),
	title TEXT,
	description TEXT,
	price TEXT,
	location TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	expires_at DATETIME,
	moderation_photo_message_ids TEXT,
	moderation_message_id INTEGER
  )`)
	if err != nil {
		log.Fatalf("Failed to create posts table: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS photos (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	file_id TEXT NOT NULL
  )`)
	if err != nil {
		log.Fatalf("Failed to create photos table: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS config (
	key TEXT PRIMARY KEY,
	value TEXT
  )`)
	if err != nil {
		log.Fatalf("Failed to create config table: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY,
	username TEXT
  )`)
	if err != nil {
		log.Fatalf("Failed to create users table: %v", err)
	}

	// Ensure all .env keys are present in config table
	envKeys := []string{
		"TELEGRAM_TOKEN",
		"MODERATION_GROUP_ID",
		"APPROVED_GROUP_ID",
		"MODERATION_TOPIC_ID",
		"APPROVED_TOPIC_ID",
		"LANG",
		"TIMEOUT_MINUTES",
		"ADMINS",
	}
	for _, key := range envKeys {
		val := os.Getenv(key)
		if val == "" {
			// Provide defaults for some keys if not set
			switch key {
			case "TIMEOUT_MINUTES":
				val = "1440"
			case "LANG":
				val = "en"
			}
		}
		if err := gosaledb.SetConfig(db, key, val); err != nil {
			log.Printf("Failed to set %s in config: %v", key, err)
		}
	}

	// Read config values from DB
	modGroup, err = gosaledb.GetConfig(db, "MODERATION_GROUP_ID")
	if err != nil {
		log.Fatal("MODERATION_GROUP_ID not set in config table")
	}
	approvedGroup, err = gosaledb.GetConfig(db, "APPROVED_GROUP_ID")
	if err != nil {
		log.Fatal("APPROVED_GROUP_ID not set in config table")
	}
	timeoutStr, err := gosaledb.GetConfig(db, "TIMEOUT_MINUTES")
	if err != nil {
		log.Fatal("TIMEOUT_MINUTES not set in config table")
	}
	timeoutMinutes, err := strconv.Atoi(timeoutStr)
	if err != nil {
		log.Fatalf("Invalid TIMEOUT_MINUTES: %v", err)
	}
	log.Printf("Config loaded: MODERATION_GROUP_ID=%s, APPROVED_GROUP_ID=%s, TIMEOUT_MINUTES=%d", modGroup, approvedGroup, timeoutMinutes)

	opts := []bot.Option{
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
			// Handle callback queries in the default handler
			if update.CallbackQuery != nil {
				gosalebot.HandleCallbackQuery(db, *update, b, ApprovedGroupID)
				return
			}
			if update.Message != nil && update.Message.From != nil {
				userID := update.Message.From.ID
				text := update.Message.Text
				// Admin commands should always be handled, regardless of FSM state
				if strings.HasPrefix(text, "/config") || text == "/pending" || text == "/showdb" || text == "/cleardb" {
					username := update.Message.From.Username
					response := gosalebot.HandleAdminCommand(db, userID, text, username)
					msg := &bot.SendMessageParams{
						ChatID:          update.Message.Chat.ID,
						Text:            response,
						ReplyParameters: &models.ReplyParameters{MessageID: update.Message.ID},
					}
					b.SendMessage(ctx, msg)
					return
				}

				if text == "/help" {
					response := gosalebot.GetHelpMessage(userID)
					msg := &bot.SendMessageParams{
						ChatID:          update.Message.Chat.ID,
						Text:            response,
						ReplyParameters: &models.ReplyParameters{MessageID: update.Message.ID},
					}
					b.SendMessage(ctx, msg)
					return
				}
				lang := os.Getenv("LANG")
				if lang == "" {
					lang = "en"
				}
				username := ""
				if update.Message.From != nil {
					username = update.Message.From.Username
				}
				resp := gosalebot.HandleMessageWithDB(db, *update, b, ModerationGroupID, lang, username, strconv.Itoa(ModerationTopicID))
				showDoneButton := false
				if session, ok := fsm.Sessions[userID]; ok && session.State == fsm.StatePhotos {
					showDoneButton = true
				}
				if showDoneButton {
					btn := models.InlineKeyboardButton{Text: "Done", CallbackData: "done"}
					markup := &models.InlineKeyboardMarkup{
						InlineKeyboard: [][]models.InlineKeyboardButton{{btn}},
					}
					msg := &bot.SendMessageParams{
						ChatID:          update.Message.Chat.ID,
						Text:            resp,
						ReplyMarkup:     markup,
						ReplyParameters: &models.ReplyParameters{MessageID: update.Message.ID},
					}
					b.SendMessage(ctx, msg)
					return
				}
				msg := &bot.SendMessageParams{
					ChatID:          update.Message.Chat.ID,
					Text:            resp,
					ReplyParameters: &models.ReplyParameters{MessageID: update.Message.ID},
				}
				if resp != "" {
					_, err := b.SendMessage(ctx, msg)
					if err != nil {
						log.Printf("Error sending message: %v", err)
					}
				}
			}
		}),
	}

	b, err := bot.New(telegramToken, opts...)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}
	gosalebot.SetGlobalBotInstance(b)
	log.Printf("GoSaleBot started. Ready to accept Telegram updates.")

	startExpirationWorker(db, time.Duration(timeoutMinutes)*time.Minute)
	gosalebot.LoadAdminsFromEnv()
	b.Start(context.Background())
}