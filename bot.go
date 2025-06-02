package bot

import (
	"database/sql"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gosalebot/db"
	"gosalebot/fsm"
	"gosalebot/i18n"
	"log"
	"strings"
)

func HandleMessageWithDB(dbConn *sql.DB, userID int64, text string, bot *tgbotapi.BotAPI, chatID int64, messageID int, photoFileIDs []string, moderationGroupID int64, lang string) string {
	// lang := "en" // In the future, detect or store user language
	session, ok := fsm.Sessions[userID]
	if !ok {
		session = &fsm.UserSession{UserID: userID, State: fsm.StateIdle, PostData: make(map[string]interface{})}
		fsm.Sessions[userID] = session
	}

	switch session.State {
	case fsm.StateIdle:
		if text == "/start" {
			session.State = fsm.StateTitle
			return i18n.T(lang, "welcome")
		}
		return i18n.T(lang, "start")
	case fsm.StateTitle:
		session.PostData["title"] = text
		session.State = fsm.StateDescription
		return i18n.T(lang, "enter_description")
	case fsm.StateDescription:
		session.PostData["description"] = text
		session.State = fsm.StatePrice
		return i18n.T(lang, "enter_price")
	case fsm.StatePrice:
		session.PostData["price"] = text
		session.State = fsm.StateLocation
		return i18n.T(lang, "enter_location")
	case fsm.StateLocation:
		session.PostData["location"] = text
		session.State = fsm.StatePhotos
		return i18n.T(lang, "send_photos")
	case fsm.StatePhotos:
		if len(photoFileIDs) > 0 {
			photos, _ := session.PostData["photos"].([]string)
			photos = append(photos, photoFileIDs...)
			session.PostData["photos"] = photos
			// Present inline 'Done' button after photo received
			return i18n.T(lang, "photo_received")
		}
		if text == "done" {
			session.State = fsm.StatePreview
			return i18n.T(lang, "preview", session.PostData["title"], session.PostData["description"], session.PostData["price"], session.PostData["location"], len(session.PostData["photos"].([]string)))
		}
		// Present inline 'Done' button even if no photo yet
		return i18n.T(lang, "send_photo_or_done")
	case fsm.StatePreview:
		if text == "confirm" {
			// Save chat_id and message_id in postData for DB
			postData := session.PostData
			postData["chat_id"] = chatID
			postData["message_id"] = messageID
			postID, err := db.SavePostToDB(dbConn, userID, postData)
			if err != nil {
				session.State = fsm.StateIdle
				return i18n.T(lang, "failed_save")
			}
			preview := fmt.Sprintf("New Sale Post:\nTitle: %s\nDescription: %s\nPrice: %s\nLocation: %s\nStatus: pending",
				session.PostData["title"], session.PostData["description"], session.PostData["price"], session.PostData["location"])
			modMsg := tgbotapi.NewMessage(moderationGroupID, preview)
			_, err = bot.Send(modMsg)
			if photos, ok := session.PostData["photos"].([]string); ok && len(photos) > 0 {
				for _, fileID := range photos {
					photoMsg := tgbotapi.NewPhoto(moderationGroupID, tgbotapi.FileID(fileID))
					photoMsg.Caption = "Photo for post ID: " + fmt.Sprint(postID)
					_, _ = bot.Send(photoMsg)
				}
			}
			session.State = fsm.StateIdle
			if err != nil {
				return i18n.T(lang, "post_saved_failed_forward")
			}
			return i18n.T(lang, "post_submitted")
		} else if text == "cancel" {
			session.State = fsm.StateIdle
			return i18n.T(lang, "post_cancelled")
		}
		return i18n.T(lang, "send_confirm_or_cancel")
	default:
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
	// Update status
	_, err = dbConn.Exec("UPDATE posts SET status = 'approved' WHERE id = ?", postID)
	if err != nil {
		log.Printf("[ERROR] ApprovePost: failed to update status: %v", err)
		return err
	}
	// Forward post to approved group
	msg := tgbotapi.NewMessage(approvedGroupID, moderationMsg.Text+"\nStatus: approved")
	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("[ERROR] ApprovePost: failed to send approved post: %v", err)
		return err
	}
	// Forward photos
	rows, err := dbConn.Query("SELECT file_id FROM photos WHERE post_id = ?", postID)
	if err != nil {
		log.Printf("[ERROR] ApprovePost: failed to query photos: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var fileID string
			if err := rows.Scan(&fileID); err == nil {
				photoMsg := tgbotapi.NewPhoto(approvedGroupID, tgbotapi.FileID(fileID))
				photoMsg.Caption = "Approved post photo"
				_, sendErr := bot.Send(photoMsg)
				if sendErr != nil {
					log.Printf("[ERROR] ApprovePost: failed to send photo: %v", sendErr)
				}
			} else {
				log.Printf("[ERROR] ApprovePost: failed to scan photo: %v", err)
			}
		}
	}
	// Delete moderation message
	deleteMsg := tgbotapi.NewDeleteMessage(moderationMsg.Chat.ID, moderationMsg.MessageID)
	_, delErr := bot.Send(deleteMsg)
	if delErr != nil {
		log.Printf("[ERROR] ApprovePost: failed to delete moderation message: %v", delErr)
	}
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
		log.Printf("[ERROR] RejectPost: failed to notify user: %v", sendErr)
	}
	// Delete moderation message
	deleteMsg := tgbotapi.NewDeleteMessage(moderationMsg.Chat.ID, moderationMsg.MessageID)
	_, delErr := bot.Send(deleteMsg)
	if delErr != nil {
		log.Printf("[ERROR] RejectPost: failed to delete moderation message: %v", delErr)
	}
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
	// TODO: Replace with real admin check (e.g., from config or env)
	adminIDs := []int64{123456789} // Replace with your Telegram user ID(s)
	for _, id := range adminIDs {
		if userID == id {
			return true
		}
	}
	return false
}

func HandleAdminCommand(dbConn *sql.DB, userID int64, text string) string {
	if !IsAdmin(userID) {
		return "You are not authorized to use this command."
	}
	if strings.HasPrefix(text, "/config ") {
		parts := strings.SplitN(text, " ", 3)
		if len(parts) == 3 {
			key, value := parts[1], parts[2]
			err := db.SetConfig(dbConn, key, value)
			if err != nil {
				return "Failed to update config: " + err.Error()
			}
			return "Config updated: " + key + " = " + value
		}
		return "Usage: /config KEY VALUE"
	}
	if text == "/config" {
		rows, err := dbConn.Query("SELECT key, value FROM config")
		if err != nil {
			return "Failed to read config: " + err.Error()
		}
		defer rows.Close()
		var out strings.Builder
		for rows.Next() {
			var key, value string
			_ = rows.Scan(&key, &value)
			out.WriteString(key + " = " + value + "\n")
		}
		return out.String()
	}
	if text == "/pending" {
		rows, err := dbConn.Query("SELECT id, user_id, title, created_at FROM posts WHERE status = 'pending'")
		if err != nil {
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
		return out.String()
	}
	return "Unknown admin command."
}
