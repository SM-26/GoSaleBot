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
	ModerationTopicID int
	ApprovedTopicID   int
)

func startExpirationWorker(db *sql.DB, interval time.Duration) {
	go func() {
		for {
			rows, err := db.Query(`SELECT id, user_id, title FROM posts WHERE status = ? AND expires_at < datetime('now')`, gosalebot.StatusPending)
			if err != nil {
				log.Printf("[ERROR] Failed to query for expired posts: %v", err)
			} else {
				for rows.Next() {
					var id, userID int64
					var title string
					if err := rows.Scan(&id, &userID, &title); err == nil {
						log.Printf("Post expired: id=%d, user_id=%d, title=%s", id, userID, title)
						// Optionally, notify admin or update status
					}
				}
				if err := rows.Close(); err != nil {
					log.Printf("[WARN] failed to close rows: %v", err)
				}
			}
			time.Sleep(interval)
		}
	}()
}

func getConfigValue(db *sql.DB, key, defaultValue string) string {
	// 1. Check environment variable
	value := os.Getenv(key)
	if value != "" {
		// If found in env, save to DB for consistency and return
		if err := gosaledb.SetConfig(db, key, value); err != nil {
			log.Printf("Failed to set config key %s from env to DB: %v", key, err)
		}
		return value
	}

	// 2. Check database
	value, err := gosaledb.GetConfig(db, key)
	if err == nil && value != "" {
		// If found in DB, set to env for consistency and return
		if err := os.Setenv(key, value); err != nil {
			log.Printf("[ERROR] Could not set environment variable %s: %v", key, err)
		}
		return value
	}

	// 3. Use default value
	if defaultValue != "" {
		// If default value is provided, set it in env and DB
		if err := os.Setenv(key, defaultValue); err != nil {
			log.Printf("[WARN] Failed to set default env for key %s: %v", key, err)
		}
		if err := gosaledb.SetConfig(db, key, defaultValue); err != nil {
			log.Printf("Failed to set default config for key %s to DB: %v", key, err)
		}
		return defaultValue
	}

	return ""
}

// No longer needed: handleUpdate. All update handling is now done via github.com/go-telegram/bot handlers.

func main() {
	db, err := sql.Open("sqlite3", "./data/gosalebot.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	// Enable foreign key support for SQLite
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		log.Fatalf("Failed to enable foreign keys: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("[ERROR] Failed to close database: %v", err)
		}
	}()
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

	// Load all config from env to DB
	configDefaults := map[string]string{
		"TELEGRAM_TOKEN":      "",
		"MODERATION_GROUP_ID": "",
		"APPROVED_GROUP_ID":   "",
		"MODERATION_TOPIC_ID": "0",
		"APPROVED_TOPIC_ID":   "0",
		"LANG":                "en",
		"TIMEOUT_MINUTES":     "1440",
		"ADMINS":              "",
		"VALIDATE_PRICE":      "true",
		"MIN_PHOTOS":          "1",
	}
	for key, defaultValue := range configDefaults {
		getConfigValue(db, key, defaultValue)
	}

	telegramToken := getConfigValue(db, "TELEGRAM_TOKEN", "")
	if telegramToken == "" {
		log.Fatal("TELEGRAM_TOKEN environment variable is required")
	}

	modGroupStr := getConfigValue(db, "MODERATION_GROUP_ID", "")
	approvedGroupStr := getConfigValue(db, "APPROVED_GROUP_ID", "")
	if modGroupStr == "" || approvedGroupStr == "" {
		log.Fatal("MODERATION_GROUP_ID and APPROVED_GROUP_ID are required")
	}

	ModerationGroupID, err = strconv.ParseInt(modGroupStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid MODERATION_GROUP_ID: %v", err)
	}
	ApprovedGroupID, err = strconv.ParseInt(approvedGroupStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid APPROVED_GROUP_ID: %v", err)
	}

	moderationTopicStr := getConfigValue(db, "MODERATION_TOPIC_ID", "0")
	ModerationTopicID, err = strconv.Atoi(moderationTopicStr)
	if err != nil {
		log.Printf("[WARNING] Invalid MODERATION_TOPIC_ID '%s', defaulting to 0: %v", moderationTopicStr, err)
		ModerationTopicID = 0
	}

	approvedTopicStr := getConfigValue(db, "APPROVED_TOPIC_ID", "0")
	ApprovedTopicID, err = strconv.Atoi(approvedTopicStr)
	if err != nil {
		log.Printf("[WARNING] Invalid APPROVED_TOPIC_ID '%s', defaulting to 0: %v", approvedTopicStr, err)
		ApprovedTopicID = 0
	}

	timeoutStr := getConfigValue(db, "TIMEOUT_MINUTES", "1440")
	timeoutMinutes, _ := strconv.Atoi(timeoutStr)

	log.Printf("Config loaded: MODERATION_GROUP_ID=%s, APPROVED_GROUP_ID=%s, TIMEOUT_MINUTES=%d", modGroupStr, approvedGroupStr, timeoutMinutes)

	opts := []bot.Option{
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
			// Handle callback queries in the default handler
			if update.CallbackQuery != nil {
				gosalebot.HandleCallbackQuery(db, *update, b, ApprovedGroupID, ApprovedTopicID)
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
					if _, err := b.SendMessage(ctx, msg); err != nil {
						log.Printf("[ERROR] Failed to send message to user: %v", err)
					}
					return
				}

				if text == "/help" {
					response := gosalebot.GetHelpMessage(userID)
					msg := &bot.SendMessageParams{
						ChatID:          update.Message.Chat.ID,
						Text:            response,
						ReplyParameters: &models.ReplyParameters{MessageID: update.Message.ID},
					}
					if _, err := b.SendMessage(ctx, msg); err != nil {
						log.Printf("[ERROR] Failed to send message to user: %v", err)
					}
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
					if _, err := b.SendMessage(ctx, msg); err != nil {
						log.Printf("[ERROR] Failed to send message to user: %v", err)
					}
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
