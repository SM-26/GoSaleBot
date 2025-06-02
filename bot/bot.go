package bot

import (
	"database/sql"
	"fmt"
	"gosalebot/db"
	"gosalebot/fsm"
	"gosalebot/i18n"
	"log"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/matterbridge/telegram-bot-api/v6"
	// tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var adminIDs map[int64]struct{}

func LoadAdminsFromEnv() {
	adminIDs = make(map[int64]struct{})
	adminsEnv := os.Getenv("ADMINS")
	for _, idStr := range strings.Split(adminsEnv, ",") {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			adminIDs[id] = struct{}{}
		}
	}
}

func HandleMessageWithDB(dbConn *sql.DB, userID int64, text string, bot *tgbotapi.BotAPI, chatID int64, messageID int, photoFileIDs []string, moderationGroupID int64, lang string, username ...string) string {
	// lang := "en" // In the future, detect or store user language
	session, ok := fsm.Sessions[userID]
	if !ok {
		session = &fsm.UserSession{UserID: userID, State: fsm.StateIdle, PostData: make(map[string]interface{})}
		fsm.Sessions[userID] = session
		log.Printf("[INFO] New session created for user %d", userID)
	}

	saveUsername := ""
	if len(username) > 0 {
		saveUsername = username[0]
	}

	switch session.State {
	case fsm.StateIdle:
		if text == "/start" {
			_, err := dbConn.Exec(`INSERT OR IGNORE INTO users (id, username) VALUES (?, ?)`, userID, saveUsername)
			if err != nil {
				log.Printf("[ERROR] Failed to insert user %d: %v", userID, err)
			} else {
				log.Printf("[INFO] User %d started bot. Username: %s", userID, saveUsername)
			}
			session.State = fsm.StateTitle
			return i18n.T(lang, "welcome")
		}
		if chatID > 0 {
			log.Printf("[INFO] User %d prompted with /start in private chat", userID)
			return i18n.T(lang, "start")
		}
		log.Printf("[WARNING] Ignored message from user %d in group/channel", userID)
		return ""
	case fsm.StateTitle:
		log.Printf("[INFO] User %d entered title: %s", userID, text)
		session.PostData["title"] = text
		session.State = fsm.StateDescription
		return i18n.T(lang, "enter_description")
	case fsm.StateDescription:
		log.Printf("[INFO] User %d entered description: %s", userID, text)
		session.PostData["description"] = text
		session.State = fsm.StatePrice
		return i18n.T(lang, "enter_price")
	case fsm.StatePrice:
		log.Printf("[INFO] User %d entered price: %s", userID, text)
		session.PostData["price"] = text
		session.State = fsm.StateLocation
		return i18n.T(lang, "enter_location")
	case fsm.StateLocation:
		log.Printf("[INFO] User %d entered location: %s", userID, text)
		session.PostData["location"] = text
		session.State = fsm.StatePhotos
		return i18n.T(lang, "send_photos")
	case fsm.StatePhotos:
		if len(photoFileIDs) > 0 {
			log.Printf("[INFO] User %d sent %d photo(s)", userID, len(photoFileIDs))
			var photos []string
			if existingPhotos, ok := session.PostData["photos"].([]string); ok {
				photos = existingPhotos
			} else {
				photos = []string{}
			}
			photos = append(photos, photoFileIDs...)
			session.PostData["photos"] = photos
			return i18n.T(lang, "photo_received")
		}
		if text == "done" {
			title := session.PostData["title"]
			description := session.PostData["description"]
			price := session.PostData["price"]
			location := session.PostData["location"]
			photos, _ := session.PostData["photos"].([]string)
			res, err := dbConn.Exec(`INSERT INTO posts (user_id, chat_id, message_id, status, title, description, price, location, created_at, expires_at) VALUES (?, ?, ?, 'pending', ?, ?, ?, ?, datetime('now'), datetime('now', '+1 day'))`,
				userID, chatID, messageID, title, description, price, location)
			if err != nil {
				log.Printf("[ERROR] Failed to insert post: %v", err)
				return i18n.T(lang, "failed_save")
			}
			postID, _ := res.LastInsertId()
			for _, fileID := range photos {
				_, err := dbConn.Exec(`INSERT INTO photos (post_id, file_id) VALUES (?, ?)`, postID, fileID)
				if err != nil {
					log.Printf("[ERROR] Failed to insert photo: %v", err)
				}
			}
			log.Printf("[INFO] Post submitted by user %d (postID: %d) for moderation", userID, postID)
			moderationMsg := i18n.T(lang, "moderation_preview", title, description, price, location)
			msg := tgbotapi.NewMessage(moderationGroupID, moderationMsg)
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("✅ Approve", "approve"),
					tgbotapi.NewInlineKeyboardButtonData("❌ Reject", "reject"),
				),
			)
			_, _ = bot.Send(msg)
			session.State = fsm.StateIdle
			session.PostData = make(map[string]interface{})
			return i18n.T(lang, "post_submitted")
		}
		log.Printf("[WARNING] User %d sent invalid input in photo state: %s", userID, text)
		return i18n.T(lang, "send_photo_or_done")
	default:
		log.Printf("[WARNING] Session reset for user %d due to unknown state", userID)
		session.State = fsm.StateIdle
		return i18n.T(lang, "session_reset")
	}
}

func ApprovePost(dbConn *sql.DB, bot *tgbotapi.BotAPI, moderationMsg *tgbotapi.Message, approvedGroupID int64) error {
	// Find the post by matching the moderation message text (could be improved with DB linkage)
	// For demo, extract title from message and find post
	title := extractTitleFromModerationMsg(moderationMsg.Text)
	row := dbConn.QueryRow("SELECT id, user_id, description, price, location FROM posts WHERE title = ? AND status = 'pending' ORDER BY created_at DESC LIMIT 1", title)
	var postID, userID int64
	var description, price, location string
	err := row.Scan(&postID, &userID, &description, &price, &location)
	if err != nil {
		log.Printf("[ERROR] ApprovePost: failed to find post for title '%s': %v", title, err)
		return err
	}
	_, err = dbConn.Exec("UPDATE posts SET status = 'approved' WHERE id = ?", postID)
	if err != nil {
		log.Printf("[ERROR] ApprovePost: failed to update status: %v", err)
		return err
	}
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = "en"
	}
	// Find the username from the DB
	var username string
	row = dbConn.QueryRow("SELECT username FROM users WHERE id = ?", userID)
	err = row.Scan(&username)
	if err != nil {
		log.Printf("[WARNING] ApprovePost: failed to find username for userID '%d': %v", userID, err)
	}
	// Compose a new message for the approved group
	var postedBy string
	if username != "" {
		// Only allow safe Telegram usernames (alphanumeric and underscores)
		if isSafeUsername(username) {
			postedBy = "@" + username
		} else {
			postedBy = fmt.Sprintf("[user](tg://user?id=%d)", userID)
		}
	} else {
		postedBy = fmt.Sprintf("[user](tg://user?id=%d)", userID)
	}
	msgText := escapeMarkdown(i18n.T(lang, "for_sale", title, description, price, location, postedBy))
	msg := tgbotapi.NewMessage(approvedGroupID, msgText)
	msg.ParseMode = "MarkdownV2"
	// If a topic ID is provided in config, set it
	topicIDStr, err := db.GetConfig(dbConn, "APPROVED_TOPIC_ID")
	var topicID int
	if err == nil && topicIDStr != "" {
		topicID, err = strconv.Atoi(topicIDStr)
		if err == nil {
			msg.MessageThreadID = topicID
		}
	}
	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("[ERROR] ApprovePost: failed to send approved post: %v", err)
		return err
	}
	// Forward photos
	rows, err := dbConn.Query("SELECT file_id FROM photos WHERE post_id = ?", postID)
	if err != nil {
		log.Printf("[WARNING] ApprovePost: failed to query photos: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var fileID string
			if err := rows.Scan(&fileID); err == nil {
				photoMsg := tgbotapi.NewPhoto(approvedGroupID, tgbotapi.FileID(fileID))
				photoMsg.Caption = "Approved post photo"
				if topicID != 0 {
					photoMsg.MessageThreadID = topicID
				}
				_, sendErr := bot.Send(photoMsg)
				if sendErr != nil {
					log.Printf("[WARNING] ApprovePost: failed to send photo: %v", sendErr)
				}
			} else {
				log.Printf("[WARNING] ApprovePost: failed to scan photo: %v", err)
			}
		}
	}
	// Delete moderation message
	deleteMsg := tgbotapi.NewDeleteMessage(moderationMsg.Chat.ID, moderationMsg.MessageID)
	_, delErr := bot.Request(deleteMsg)
	if delErr != nil {
		log.Printf("[WARNING] ApprovePost: failed to delete moderation message: %v", delErr)
	}
	log.Printf("[INFO] Post %d approved and published by admin", postID)
	return nil
}

func RejectPost(dbConn *sql.DB, bot *tgbotapi.BotAPI, moderationMsg *tgbotapi.Message, replyText string) error {
	title := extractTitleFromModerationMsg(moderationMsg.Text)
	row := dbConn.QueryRow("SELECT id, user_id FROM posts WHERE title = ? AND status = 'pending' ORDER BY created_at DESC LIMIT 1", title)
	var postID, userID int64
	err := row.Scan(&postID, &userID)
	if err != nil {
		log.Printf("[ERROR] RejectPost: failed to find post for title '%s': %v", title, err)
		return err
	}
	_, err = dbConn.Exec("UPDATE posts SET status = 'rejected' WHERE id = ?", postID)
	if err != nil {
		log.Printf("[ERROR] RejectPost: failed to update status: %v", err)
		return err
	}
	msg := tgbotapi.NewMessage(userID, "Your post was rejected: "+replyText)
	_, sendErr := bot.Send(msg)
	if sendErr != nil {
		log.Printf("[WARNING] RejectPost: failed to notify user: %v", sendErr)
	}
	// Delete moderation message
	deleteMsg := tgbotapi.NewDeleteMessage(moderationMsg.Chat.ID, moderationMsg.MessageID)
	_, delErr := bot.Request(deleteMsg)
	if delErr != nil {
		log.Printf("[WARNING] RejectPost: failed to delete moderation message: %v", delErr)
	}
	log.Printf("[INFO] Post %d rejected by admin", postID)
	return nil
}

func extractTitleFromModerationMsg(text string) string {
	// Assumes the title is on the line starting with "Title: "
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "Title: ") {
			return strings.TrimPrefix(line, "Title: ")
		}
	}
	return ""
}

func IsAdmin(userID int64) bool {
	_, ok := adminIDs[userID]
	return ok
}

func HandleAdminCommand(dbConn *sql.DB, userID int64, text string) string {
	if !IsAdmin(userID) {
		log.Printf("[WARNING] Unauthorized admin command attempt by user %d", userID)
		return "You are not authorized to use this command."
	}
	if strings.HasPrefix(text, "/config ") {
		parts := strings.SplitN(text, " ", 3)
		if len(parts) == 3 {
			key, value := parts[1], parts[2]
			err := db.SetConfig(dbConn, key, value)
			if err != nil {
				log.Printf("[ERROR] Failed to update config %s: %v", key, err)
				return "Failed to update config: " + err.Error()
			}
			log.Printf("[INFO] Config updated by admin %d: %s = %s", userID, key, value)
			return "Config updated: " + key + " = " + value
		}
		log.Printf("[WARNING] Invalid /config usage by admin %d", userID)
		return "Usage: /config KEY VALUE"
	}
	if text == "/config" {
		rows, err := dbConn.Query("SELECT key, value FROM config")
		if err != nil {
			log.Printf("[ERROR] Failed to read config: %v", err)
			return "Failed to read config: " + err.Error()
		}
		defer rows.Close()
		var out strings.Builder
		for rows.Next() {
			var key, value string
			_ = rows.Scan(&key, &value)
			out.WriteString(key + " = " + value + "\n")
		}
		log.Printf("[INFO] Admin %d listed config", userID)
		return out.String()
	}
	if text == "/pending" {
		rows, err := dbConn.Query("SELECT id, user_id, title, created_at FROM posts WHERE status = 'pending'")
		if err != nil {
			log.Printf("[ERROR] Failed to query pending posts: %v", err)
			return "Failed to query pending posts: " + err.Error()
		}
		defer rows.Close()
		var out strings.Builder
		for rows.Next() {
			var id, userID int64
			var title, createdAt string
			_ = rows.Scan(&id, &userID, &title, &createdAt)
			out.WriteString(fmt.Sprintf("ID: %d, User: %d, Title: %s, Created: %s\n", id, userID, title, createdAt))
		}
		log.Printf("[INFO] Admin %d listed pending posts", userID)
		return out.String()
	}
	log.Printf("[WARNING] Unknown admin command by user %d: %s", userID, text)
	return "Unknown admin command."
}

func HandleCallbackQuery(db *sql.DB, update tgbotapi.Update, botAPI *tgbotapi.BotAPI, approvedGroupID int64) {
	if update.CallbackQuery != nil {
		data := update.CallbackQuery.Data
		lang := os.Getenv("LANG")
		if lang == "" {
			lang = "en"
		}
		userID := update.CallbackQuery.From.ID
		if data == "approve" {
			log.Printf("[INFO] Admin %d approved a post via inline button", userID)
			_ = ApprovePost(db, botAPI, update.CallbackQuery.Message, approvedGroupID)
		} else if data == "reject" {
			log.Printf("[INFO] Admin %d rejected a post via inline button", userID)
			_ = RejectPost(db, botAPI, update.CallbackQuery.Message, "Rejected by admin")
		}
		return
	}
}

// Add these helpers at the end of the file:
func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
		"'", "\\'",
		"\"", "\\\"",
	)
	return replacer.Replace(s)
}

func isSafeUsername(username string) bool {
	for _, r := range username {
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}
