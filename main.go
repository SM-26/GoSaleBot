package main

import (
	"database/sql"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"gosalebot/bot"
	gosaledb "gosalebot/db"
	"gosalebot/fsm"
	"gosalebot/i18n"

	tgbotapi "github.com/matterbridge/telegram-bot-api/v6" // <--- ADDED THIS LINE
	_ "github.com/mattn/go-sqlite3"                        // <--- Likely needed for your DB connection
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

func handleUpdate(db *sql.DB, botAPI *tgbotapi.BotAPI, update tgbotapi.Update, moderationGroupID, approvedGroupID int64) {
	if update.CallbackQuery != nil {
		data := update.CallbackQuery.Data
		userID := update.CallbackQuery.From.ID
		chatID := update.CallbackQuery.Message.Chat.ID
		messageID := update.CallbackQuery.Message.MessageID
		lang := os.Getenv("LANG")
		if lang == "" {
			lang = "en"
		}
		// Only handle confirm/cancel for user preview here
		if data == "confirm" || data == "cancel" {
			resp := bot.HandleMessageWithDB(db, userID, data, botAPI, chatID, messageID, nil, moderationGroupID, lang)
			edit := tgbotapi.NewEditMessageText(chatID, messageID, resp)
			botAPI.Send(edit)
			return
		} else if data == "done" {
			if session, ok := fsm.Sessions[userID]; ok && session.State == fsm.StatePhotos {
				response := bot.HandleMessageWithDB(db, userID, "done", botAPI, chatID, messageID, nil, moderationGroupID, lang)
				edit := tgbotapi.NewEditMessageText(chatID, messageID, response)
				botAPI.Send(edit)
			}
			return
		} else if data == "approve" {
			err := bot.ApprovePost(db, botAPI, update.CallbackQuery.Message, approvedGroupID)
			if err != nil {
				log.Printf("[ERROR] Failed to approve post: %v", err)
				edit := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Failed to approve post.")
				botAPI.Send(edit)
			} else {
				edit := tgbotapi.NewEditMessageText(chatID, messageID, "✅ Approved and forwarded.")
				botAPI.Send(edit)
			}
			return
		} else if data == "reject" {
			err := bot.RejectPost(db, botAPI, update.CallbackQuery.Message, "Rejected by admin")
			if err != nil {
				log.Printf("[ERROR] Failed to reject post: %v", err)
			}
			edit := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Rejected.")
			botAPI.Send(edit)
			return
		}
	}

	if update.Message != nil && update.Message.From != nil {
		userID := update.Message.From.ID
		text := update.Message.Text
		var photoFileIDs []string
		if update.Message.Photo != nil && len(update.Message.Photo) > 0 {
			for _, photo := range update.Message.Photo {
				photoFileIDs = append(photoFileIDs, photo.FileID)
			}
		}
		if update.Message.Chat.ID == moderationGroupID {
			if update.Message.ReplyToMessage != nil {
				err := bot.RejectPost(db, botAPI, update.Message.ReplyToMessage, update.Message.Text)
				if err != nil {
					log.Printf("[ERROR] Failed to reject post: %v", err)
				}
				return
			}
			if text == "/approve" || text == "✅" {
				err := bot.ApprovePost(db, botAPI, update.Message, approvedGroupID)
				if err != nil {
					log.Printf("[ERROR] Failed to approve post: %v", err)
				}
				return
			}
		}
		lang := os.Getenv("LANG")
		if lang == "" {
			lang = "en"
		}
		if bot.IsAdmin(userID) && (strings.HasPrefix(text, "/config") || text == "/pending") {
			response := bot.HandleAdminCommand(db, userID, text)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, response)
			msg.ReplyToMessageID = update.Message.MessageID
			botAPI.Send(msg)
			return
		}
		var session *fsm.UserSession
		if s, ok := fsm.Sessions[userID]; ok {
			session = s
		}
		username := ""
		if update.Message.From != nil {
			username = update.Message.From.UserName
		}
		resp := bot.HandleMessageWithDB(db, userID, text, botAPI, update.Message.Chat.ID, update.Message.MessageID, photoFileIDs, moderationGroupID, lang, username)
		if text == "done" && session != nil && resp == i18n.T(lang, "preview", session.PostData["title"], session.PostData["description"], session.PostData["price"], session.PostData["location"], len(session.PostData["photos"].([]string))) {
			// User just finished photo upload, now in preview state: show preview with inline buttons
			if session.State == fsm.StatePreview {
				var numPhotos int
				if photos, ok := session.PostData["photos"].([]string); ok {
					numPhotos = len(photos)
				} else if photos, ok := session.PostData["photos"].([]interface{}); ok {
					numPhotos = len(photos)
				} else {
					numPhotos = 0
				}
				preview := i18n.T(lang, "preview",
					session.PostData["title"], session.PostData["description"],
					session.PostData["price"], session.PostData["location"], numPhotos,
				)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, preview)
				msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("✅ Confirm", "confirm"),
						tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel"),
					),
				)
				msg.ReplyToMessageID = update.Message.MessageID
				botAPI.Send(msg)
				return
			}
		}
		showDoneButton := false
		if session, ok := fsm.Sessions[userID]; ok && session.State == fsm.StatePhotos {
			showDoneButton = true
		}
		if showDoneButton {
			btn := tgbotapi.NewInlineKeyboardButtonData("Done", "done")
			markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, resp)
			msg.ReplyMarkup = markup
			msg.ReplyToMessageID = update.Message.MessageID
			botAPI.Send(msg)
			return
		}
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, resp)
		msg.ReplyToMessageID = update.Message.MessageID
		if resp != "" {
			_, err := botAPI.Send(msg)
			if err != nil {
				log.Printf("Error sending message: %v", err)
			}
		}
	}
}

func main() {
	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	if telegramToken == "" {
		log.Fatal("TELEGRAM_TOKEN environment variable is required")
	}

	modGroup := os.Getenv("MODERATION_GROUP_ID")
	approvedGroup := os.Getenv("APPROVED_GROUP_ID")
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

	db, err := sql.Open("sqlite3", "./data/gosalebot.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
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
		expires_at DATETIME
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

	// Set config values from env if not present
	if err := gosaledb.SetConfig(db, "MODERATION_GROUP_ID", modGroup); err != nil {
		log.Printf("Failed to set MODERATION_GROUP_ID in config: %v", err)
	}
	if err := gosaledb.SetConfig(db, "APPROVED_GROUP_ID", approvedGroup); err != nil {
		log.Printf("Failed to set APPROVED_GROUP_ID in config: %v", err)
	}
	if err := gosaledb.SetConfig(db, "TIMEOUT_MINUTES", "1440"); err != nil { // default 24h
		log.Printf("Failed to set TIMEOUT_MINUTES in config: %v", err)
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

	botAPI, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}
	log.Printf("Authorized on account %s", botAPI.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := botAPI.GetUpdatesChan(u)

	startExpirationWorker(db, time.Duration(timeoutMinutes)*time.Minute)

	bot.LoadAdminsFromEnv()

	for update := range updates {
		handleUpdate(db, botAPI, update, ModerationGroupID, ApprovedGroupID)
	}
	log.Println("GoSaleBot started. Ready to accept Telegram updates.")
}
