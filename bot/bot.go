package bot

import (
	"context"
	"database/sql"
	"fmt"
	"gosalebot/db"
	"gosalebot/fsm"
	"gosalebot/i18n"
	"log"
	"os"
	"strconv"
	"strings"

	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
)

// globalBotInstance is set at startup in main.go for admin command use
var globalBotInstance *telegram.Bot
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

func SetGlobalBotInstance(b *telegram.Bot) {
	globalBotInstance = b
}

func HandleMessageWithDB(dbConn *sql.DB, update models.Update, bot *telegram.Bot, moderationGroupID int64, lang string, username ...string) string {
	// Ensure update.Message is not nil before proceeding
	if update.Message == nil {
		return ""
	}

	// Handle replies to moderation messages for rejection reasons
	if update.Message.ReplyToMessage != nil && IsAdmin(update.Message.From.ID) {
		repliedToMsgID := update.Message.ReplyToMessage.ID
		repliedToChatID := update.Message.ReplyToMessage.Chat.ID

		// Check if the replied-to message is a moderation message from the bot
		if repliedToChatID == moderationGroupID {
			var postID int64
			row := dbConn.QueryRow("SELECT id FROM posts WHERE moderation_message_id = ?", repliedToMsgID)
			err := row.Scan(&postID)
			if err == nil {
				// This is a reply to a moderation message, use the reply text as rejection reason
				log.Printf("[INFO] Admin %d rejected post %d via reply with reason: %s", update.Message.From.ID, postID, update.Message.Text)
				err := RejectPost(dbConn, update.Message.ReplyToMessage, update.Message.Text, postID)
				if err != nil {
					log.Printf("[ERROR] Failed to reject post %d via reply: %v", postID, err)
					return "Failed to reject post."
				}
				return "Post rejected with custom reason."
			}
		}
	}

	// lang := "en" // In the future, detect or store user language
	userID := update.Message.From.ID
	text := update.Message.Text
	chatID := update.Message.Chat.ID
	messageID := update.Message.ID
	var photoFileIDs []string
	if len(update.Message.Photo) > 0 {
		// Only use the largest photo (last in the array)
		photo := update.Message.Photo[len(update.Message.Photo)-1]
		photoFileIDs = append(photoFileIDs, photo.FileID)
	}
	session, ok := fsm.Sessions[userID]
	if !ok {
		session = &fsm.UserSession{UserID: userID, State: fsm.StateIdle, PostData: make(map[string]interface{})}
		fsm.Sessions[userID] = session
		log.Printf("[INFO] New session created for user %d", userID)
	}

	var saveUsername string
	if len(username) > 0 && username[0] != "" {
		saveUsername = username[0]
	} else if update.Message.From != nil {
		saveUsername = strings.TrimSpace(update.Message.From.FirstName + " " + update.Message.From.LastName)
	}

	// Optionally accept ModerationTopicID as last variadic argument (as string)
	moderationTopicID := 0
	if len(username) > 1 {
		// username[1] is expected to be a string containing the topic ID
		moderationTopicID, _ = strconv.Atoi(username[1])
	}
	switch session.State {
	case fsm.StateIdle:
		{
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
		}
	case fsm.StateTitle:
		{
			log.Printf("[INFO] User %d entered title: %s", userID, text)
			session.PostData["title"] = text
			session.State = fsm.StateDescription
			return i18n.T(lang, "enter_description")
		}
	case fsm.StateDescription:
		{
			log.Printf("[INFO] User %d entered description: %s", userID, text)
			session.PostData["description"] = text
			session.State = fsm.StatePrice
			return i18n.T(lang, "enter_price")
		}
	case fsm.StatePrice:
		log.Printf("[INFO] User %d entered price: %s", userID, text)
		session.PostData["price"] = text
		session.State = fsm.StateLocation
		return i18n.T(lang, "enter_location")
	case fsm.StateLocation:
		{
			log.Printf("[INFO] User %d entered location: %s", userID, text)
			session.PostData["location"] = text
			session.State = fsm.StatePhotos
			return i18n.T(lang, "send_photos")
		}
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
		if strings.ToLower(text) == "done" {
			title := session.PostData["title"]
			description := session.PostData["description"]
			price := session.PostData["price"]
			location := session.PostData["location"]
			photos, _ := session.PostData["photos"].([]string)
			// Always store the user's chat for the post, but send moderation message to moderationGroupID
			res, err := dbConn.Exec(`INSERT INTO posts (user_id, chat_id, message_id, status, title, description, price, location, created_at, expires_at, moderation_message_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now', '+1 day'), ?)`, userID, chatID, messageID, StatusPending, title, description, price, location, 0)
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
			postedBy := formatPostedBy(saveUsername, userID)
			moderationMsg := i18n.T(lang, "moderation_preview_with_id", postID, title, description, price, location, postedBy)
			replyMarkup := &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{
					{
						{Text: "✅ Approve", CallbackData: fmt.Sprintf("approve:%d", postID)},
						{Text: "❌ Reject", CallbackData: fmt.Sprintf("reject:%d", postID)},
					},
				},
			}
			ctx := context.Background()
			if len(photos) > 0 {
				// 1. Send moderation message with inline buttons (no photo)
				msgParams := &telegram.SendMessageParams{
					ChatID:      moderationGroupID,
					Text:        moderationMsg,
					ReplyMarkup: replyMarkup,
				}
				if moderationTopicID != 0 {
					msgParams.MessageThreadID = moderationTopicID
				}
				sentModMsg, err := bot.SendMessage(ctx, msgParams)
				if err != nil {
					log.Printf("[ERROR] Failed to send moderation message: %v", err)
				} else {
					_, err := dbConn.Exec("UPDATE posts SET moderation_message_id = ? WHERE id = ?", sentModMsg.ID, postID)
					if err != nil {
						log.Printf("[ERROR] Failed to update moderation_message_id: %v", err)
					}
				}
				// 2. Send each photo as a follow-up message (no caption/buttons)
				var moderationPhotoMessageIDs []string
				for _, fileID := range photos {
					photoParams := &telegram.SendPhotoParams{
						ChatID: moderationGroupID,
						Photo:  &models.InputFileString{Data: fileID},
					}
					if moderationTopicID != 0 {
						photoParams.MessageThreadID = moderationTopicID
					}
					sentMsg, err := bot.SendPhoto(ctx, photoParams)
					if err != nil {
						log.Printf("[ERROR] Failed to send moderation photo: %v", err)
					} else {
						moderationPhotoMessageIDs = append(moderationPhotoMessageIDs, strconv.Itoa(sentMsg.ID))
					}
				}
				if len(moderationPhotoMessageIDs) > 0 {
					_, err := dbConn.Exec("UPDATE posts SET moderation_photo_message_ids = ? WHERE id = ?", strings.Join(moderationPhotoMessageIDs, ","), postID)
					if err != nil {
						log.Printf("[ERROR] Failed to update moderation_photo_message_ids: %v", err)
					}
				}
			} else {
				// Declare and initialize msgParams here
				msgParams := &telegram.SendMessageParams{
					ChatID:      moderationGroupID, // send to moderation group, not user chat
					Text:        moderationMsg,
					ReplyMarkup: replyMarkup,
				}
				if moderationTopicID != 0 {
					msgParams.MessageThreadID = moderationTopicID
				}
				sentModMsg, err := bot.SendMessage(ctx, msgParams)
				if err != nil {
					log.Printf("[ERROR] Failed to send moderation message: %v", err)
				} else {
					_, err := dbConn.Exec("UPDATE posts SET moderation_message_id = ? WHERE id = ?", sentModMsg.ID, postID)
					if err != nil {
						log.Printf("[ERROR] Failed to update moderation_message_id: %v", err)
					}
				}
			}
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

// ApprovePost publishes a post to the approved group and deletes the moderation message.
func ApprovePost(dbConn *sql.DB, moderationMsg *models.Message, approvedGroupID int64, postID int64) error {
	var userID int64
	var title, description, price, location, status string
	var moderationPhotoMessageIDsStr sql.NullString
	row := dbConn.QueryRow("SELECT user_id, title, description, price, location, status, moderation_photo_message_ids FROM posts WHERE id = ?", postID)
	err := row.Scan(&userID, &title, &description, &price, &location, &status, &moderationPhotoMessageIDsStr)
	if err != nil {
		log.Printf("[ERROR] ApprovePost: failed to find post for id %d: %v", postID, err)
		return err
	}

	if status != StatusPending {
		log.Printf("[INFO] ApprovePost: post %d is not pending (status: %s), skipping approval.", postID, status)
		return nil
	}

	// Fetch all photo file_ids for the post BEFORE deleting them

	rows, err := dbConn.Query("SELECT file_id FROM photos WHERE post_id = ?", postID)
	var photoFileIDs []string
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var fileID string
			if err := rows.Scan(&fileID); err == nil {
				photoFileIDs = append(photoFileIDs, fileID)
			}
		}
	} else {
		log.Printf("[ERROR] ApprovePost: failed to query photos for post %d: %v", postID, err)
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
	title = EscapeMarkdown(title)
	description = EscapeMarkdown(description)
	price = EscapeMarkdown(price)
	location = EscapeMarkdown(location)
	postedBy := formatPostedBy(username, userID)
	msgText := i18n.T(lang, "for_sale", title, description, price, location, postedBy)

	ctx := context.Background()
	if len(photoFileIDs) > 0 {
		// Send as media group (album)
		var mediaGroup []models.InputMedia
		for i, fileID := range photoFileIDs {
			media := &models.InputMediaPhoto{
				Media: fileID,
			}
			if i == 0 {
				media.Caption = msgText
				media.ParseMode = "Markdown"
			}
			mediaGroup = append(mediaGroup, media)
		}
		mediaReq := &telegram.SendMediaGroupParams{
			ChatID: approvedGroupID,
			Media:  mediaGroup,
		}
		_, err = globalBotInstance.SendMediaGroup(ctx, mediaReq)
		if err != nil {
			log.Printf("[ERROR] ApprovePost: failed to send media group: %v", err)
		}
	} else {
		msgReq := &telegram.SendMessageParams{
			ChatID:    approvedGroupID,
			Text:      msgText,
			ParseMode: "Markdown",
		}
		_, err = globalBotInstance.SendMessage(ctx, msgReq)
		if err != nil {
			log.Printf("[ERROR] ApprovePost: failed to send approved post: %v", err)
			return err
		}
	}

	// Now that the post is sent, update the status and delete the photos from db
	_, err = dbConn.Exec("UPDATE posts SET status = ? WHERE id = ?", StatusApproved, postID)
	if err != nil {
		log.Printf("[ERROR] ApprovePost: failed to update status: %v", err)
		return err
	}

	_, err = dbConn.Exec("DELETE FROM photos WHERE post_id = ?", postID)
	if err != nil {
		log.Printf("[ERROR] ApprovePost: failed to delete photos for post %d: %v", postID, err)
	}

	// Notify the user that their post was approved
	notifyText := i18n.T(lang, "post_approved")
	notifyReq := &telegram.SendMessageParams{
		ChatID: userID,
		Text:   notifyText,
	}
	_, notifyErr := globalBotInstance.SendMessage(ctx, notifyReq)
	if notifyErr != nil {
		log.Printf("[WARNING] ApprovePost: failed to notify user %d: %v", userID, notifyErr)
	}
	// Delete moderation message
	deleteReq := &telegram.DeleteMessageParams{
		ChatID:    moderationMsg.Chat.ID,
		MessageID: moderationMsg.ID,
	}
	_, delErr := globalBotInstance.DeleteMessage(ctx, deleteReq)
	if delErr != nil {
		log.Printf("[WARNING] ApprovePost: failed to delete moderation message: %v", delErr)
	}

	// Delete all moderation group images for this post
	if moderationPhotoMessageIDsStr.Valid {
		moderationImageMsgIDs := strings.Split(moderationPhotoMessageIDsStr.String, ",")
		for _, msgIDStr := range moderationImageMsgIDs {
			msgID, err := strconv.Atoi(msgIDStr)
			if err != nil {
				log.Printf("[WARNING] ApprovePost: failed to parse moderation image message ID '%s': %v", msgIDStr, err)
				continue
			}
			imgDelReq := &telegram.DeleteMessageParams{
				ChatID:    moderationMsg.Chat.ID,
				MessageID: msgID,
			}
			_, imgDelErr := globalBotInstance.DeleteMessage(ctx, imgDelReq)
			if imgDelErr != nil {
				log.Printf("[WARNING] ApprovePost: failed to delete moderation image message %d: %v", msgID, imgDelErr)
			}
		}
	}

	log.Printf("[INFO] Post %d approved and published by admin", postID)
	return nil
}

// Helper to format a Telegram username or fallback to user mention
func formatPostedBy(username string, userID int64) string {
	if username != "" {
		if isSafeUsername(username) {
			return "@" + username
		}
		return EscapeMarkdown(username)
	}
	return fmt.Sprintf("[user](tg://user?id=%d)", userID)
}

func RejectPost(dbConn *sql.DB, moderationMsg *models.Message, replyText string, postID int64) error {
	row := dbConn.QueryRow("SELECT user_id FROM posts WHERE id = ? AND status = ?", postID, StatusPending)
	var userID int64
	err := row.Scan(&userID)
	if err != nil {
		log.Printf("[ERROR] RejectPost: failed to find post for id %d: %v", postID, err)
		return err
	}
	_, err = dbConn.Exec("UPDATE posts SET status = ? WHERE id = ?", StatusRejected, postID)
	if err != nil {
		log.Printf("[ERROR] RejectPost: failed to update status: %v", err)
		return err
	}

	// Delete photos from the database
	_, err = dbConn.Exec("DELETE FROM photos WHERE post_id = ?", postID)
	if err != nil {
		log.Printf("[ERROR] RejectPost: failed to delete photos for post %d: %v", postID, err)
		// Do not return error, as the post is already rejected
	}
	ctx := context.Background()
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = "en"
	}
	msgReq := &telegram.SendMessageParams{
		ChatID: userID,
		Text:   i18n.T(lang, "post_rejected", replyText),
	}
	_, sendErr := globalBotInstance.SendMessage(ctx, msgReq)
	if sendErr != nil {
		log.Printf("[WARNING] RejectPost: failed to notify user: %v", sendErr)
	}
	// Delete moderation message
	deleteReq := &telegram.DeleteMessageParams{
		ChatID:    moderationMsg.Chat.ID,
		MessageID: moderationMsg.ID,
	}
	_, delErr := globalBotInstance.DeleteMessage(ctx, deleteReq)
	if delErr != nil {
		log.Printf("[WARNING] RejectPost: failed to delete moderation message: %v", delErr)
	}
	log.Printf("[INFO] Post %d rejected by admin", postID)
	return nil
}

func IsAdmin(userID int64) bool {
	_, ok := adminIDs[userID]
	return ok
}

func HandleAdminCommand(dbConn *sql.DB, userID int64, text string, username string) string {
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = "en"
	}
	if !IsAdmin(userID) {
		if username != "" {
			log.Printf("[WARNING] Unauthorized admin command attempt by user %d (@%s), command: %s", userID, username, text)
		} else {
			log.Printf("[WARNING] Unauthorized admin command attempt by user %d, command: %s", userID, text)
		}
		return i18n.T(lang, "not_authorized")
	}
	// Approve or reject a pending post by ID: /pending ID approve or /pending ID reject
	if strings.HasPrefix(text, "/pending ") {
		parts := strings.Fields(text)
		if len(parts) == 3 && (parts[2] == "approve" || parts[2] == "reject") {
			postID, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return "Invalid post ID."
			}
			row := dbConn.QueryRow("SELECT title, user_id FROM posts WHERE id = ? AND status = ?", postID, StatusPending)
			var title string
			var userIDVal int64
			err = row.Scan(&title, &userIDVal)
			if err != nil {
				return "No pending post found with that ID."
			}
			lang := os.Getenv("LANG")
			if lang == "" {
				lang = "en"
			}

			if parts[2] == "approve" {
				// Approve logic (same as before)
				// Find the moderation group and approved group IDs
				// moderationGroupID, _ := strconv.ParseInt(os.Getenv("MODERATION_GROUP_ID"), 10, 64)
				approvedGroupID, _ := strconv.ParseInt(os.Getenv("APPROVED_GROUP_ID"), 10, 64)
				// Fetch post details
				row = dbConn.QueryRow("SELECT description, price, location FROM posts WHERE id = ?", postID)
				var description, price, location string
				err = row.Scan(&description, &price, &location)
				if err != nil {
					return "Failed to fetch post details."
				}
				// Mark as approved
				_, err = dbConn.Exec("UPDATE posts SET status = ? WHERE id = ?", StatusApproved, postID)
				if err != nil {
					return "Failed to update post status."
				}
				// Fetch username
				var username string
				row = dbConn.QueryRow("SELECT username FROM users WHERE id = ?", userIDVal)
				_ = row.Scan(&username)
				postedBy := formatPostedBy(username, userIDVal)
				msgText := EscapeMarkdown(i18n.T(lang, "for_sale", title, description, price, location, postedBy))
				// Fetch all photo file_ids for the post

				rows, err := dbConn.Query("SELECT file_id FROM photos WHERE post_id = ?", postID)
				var photoFileIDs []string
				if err == nil {
					defer rows.Close()
					for rows.Next() {
						var fileID string
						if err := rows.Scan(&fileID); err == nil {
							photoFileIDs = append(photoFileIDs, fileID)
						}
					}
				}
				ctx := context.Background()
				if len(photoFileIDs) > 0 {
					var mediaGroup []models.InputMedia
					for i, fileID := range photoFileIDs {
						media := &models.InputMediaPhoto{
							Media: fileID,
						}
						if i == 0 {
							media.Caption = msgText
							media.ParseMode = "Markdown"
						}
						mediaGroup = append(mediaGroup, media)
					}
					mediaReq := &telegram.SendMediaGroupParams{
						ChatID: approvedGroupID,
						Media:  mediaGroup,
					}
					_, err = globalBotInstance.SendMediaGroup(ctx, mediaReq)
					if err != nil {
						return "Failed to send approved post (media group)."
					}
				} else {
					msgReq := &telegram.SendMessageParams{
						ChatID:    approvedGroupID,
						Text:      msgText,
						ParseMode: "Markdown",
					}
					_, err = globalBotInstance.SendMessage(ctx, msgReq)
					if err != nil {
						return "Failed to send approved post."
					}
				}
				// Notify the user
				notifyText := i18n.T(lang, "post_approved")
				notifyReq := &telegram.SendMessageParams{
					ChatID: userIDVal,
					Text:   notifyText,
				}
				_, _ = globalBotInstance.SendMessage(ctx, notifyReq)
				return "Post approved and published."
			} else if parts[2] == "reject" {
				// Reject logic
				_, err = dbConn.Exec("UPDATE posts SET status = ? WHERE id = ?", StatusRejected, postID)
				if err != nil {
					return "Failed to update post status."
				}
				notifyText := i18n.T(lang, "post_rejected", "Rejected by admin")
				notifyReq := &telegram.SendMessageParams{
					ChatID: userIDVal,
					Text:   notifyText,
				}
				_, _ = globalBotInstance.SendMessage(context.Background(), notifyReq)
				return "Post rejected."
			}
		} // This is the missing brace for "if len(parts) == 3 && (parts[2] == "approve" || parts[2] == "reject")"
		// If just /pending, fall through to the normal pending list
	}
	if text == "/showdb" {

		// Show all posts, users, photos, and config in the database
		var out strings.Builder
		out.WriteString("---" + "Posts" + "---" + "\n")

		rows, err := dbConn.Query("SELECT id, user_id, chat_id, message_id, status, title, description, price, location, created_at, expires_at FROM posts")
		if err != nil {
			log.Printf("[ERROR] Failed to query posts: %v", err)
			out.WriteString("Failed to query posts: " + err.Error() + "\n")
		} else {
			defer rows.Close()
			for rows.Next() {
				var id, userID, chatID, messageID int64
				var status, title, description, price, location, createdAt, expiresAt string
				_ = rows.Scan(&id, &userID, &chatID, &messageID, &status, &title, &description, &price, &location, createdAt, expiresAt)
				out.WriteString(fmt.Sprintf("ID: %d, User: %d, Chat: %d, Msg: %d, Status: %s, Title: %s, Desc: %s, Price: %s, Loc: %s, Created: %s, Expires: %s\n",
					id, userID, chatID, messageID, status, title, description, price, location, createdAt, expiresAt))
			}
		}
		out.WriteString("---" + "Users" + "---" + "\n")
		userRows, err := dbConn.Query("SELECT id, username FROM users")
		if err != nil {
			log.Printf("[ERROR] Failed to query users: %v", err)
			out.WriteString("Failed to query users: " + err.Error() + "\n")
		} else {
			defer userRows.Close()
			for userRows.Next() {
				var id int64
				var username string
				_ = userRows.Scan(&id, &username)
				out.WriteString(fmt.Sprintf("ID: %d, Username: %s\n", id, username))
			}
		}
		out.WriteString("---" + "Photos" + "---" + "\n")
		photoRows, err := dbConn.Query("SELECT id, post_id, file_id FROM photos")
		if err != nil {
			log.Printf("[ERROR] Failed to query photos: %v", err)
			out.WriteString("Failed to query photos: " + err.Error() + "\n")
		} else {
			defer photoRows.Close()
			for photoRows.Next() {
				var id, postID int64
				var fileID string
				_ = photoRows.Scan(&id, &postID, &fileID)
				out.WriteString(fmt.Sprintf("ID: %d, PostID: %d, FileID: %s\n", id, postID, fileID))
			}
		}
		out.WriteString("---" + "Config" + "---" + "\n")
		configRows, err := dbConn.Query("SELECT key, value FROM config")
		if err != nil {
			log.Printf("[ERROR] Failed to query config: %v", err)
			out.WriteString("Failed to query config: " + err.Error() + "\n")
		} else {
			defer configRows.Close()
			for configRows.Next() {
				var key, value string
				_ = configRows.Scan(&key, &value)
				out.WriteString(fmt.Sprintf("%s = %s\n", key, value))
			}
		}
		log.Printf("[INFO] Admin %d showed db", userID)
		return out.String()
	}
	if text == "/cleardb" {
		// Clear all photos and posts from the database (do not clear users)
		_, err := dbConn.Exec("DELETE FROM photos")
		if err != nil {
			log.Printf("[ERROR] Failed to clear photos: %v", err)
			return i18n.T(lang, "failed_clear_photos", err.Error())
		}
		_, err = dbConn.Exec("DELETE FROM posts")
		if err != nil {
			log.Printf("[ERROR] Failed to clear posts: %v", err)
			return i18n.T(lang, "failed_clear_posts", err.Error())
		}
		log.Printf("[INFO] Admin %d cleared posts and photos", userID)
		return i18n.T(lang, "db_cleared")
	}
	if strings.HasPrefix(text, "/config ") {
		parts := strings.SplitN(text, " ", 3)
		if len(parts) == 3 {
			key, value := parts[1], parts[2]
			err := db.SetConfig(dbConn, key, value)
			if err != nil {
				log.Printf("[ERROR] Failed to update config %s: %v", key, err)
				return i18n.T(lang, "failed_update_config", err.Error())
			}
			log.Printf("[INFO] Config updated by admin %d: %s = %s", userID, key, value)
			return i18n.T(lang, "config_updated", key, value)
		}
		log.Printf("[WARNING] Invalid /config usage by admin %d", userID)
		return i18n.T(lang, "config_usage")
	}

	if text == "/config" {
		// List of .env keys to show
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
		var out strings.Builder
		for _, key := range envKeys {
			val, err := db.GetConfig(dbConn, key)
			if err != nil || val == "" {
				val = os.Getenv(key)
			}
			out.WriteString(key + " = " + val + "\n")
		}
		log.Printf("[INFO] Admin %d listed config", userID)
		return out.String()
	}
	if text == "/pending" {

		rows, err := dbConn.Query("SELECT id, user_id, title, created_at FROM posts WHERE status = ?", StatusPending)
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
	return i18n.T(lang, "unknown_admin_command")
}

func HandleCallbackQuery(db *sql.DB, update models.Update, botAPI *telegram.Bot, approvedGroupID int64) {
	if update.CallbackQuery != nil {
		data := update.CallbackQuery.Data
		lang := os.Getenv("LANG")
		if lang == "" {
			lang = "en"
		}
		userID := update.CallbackQuery.From.ID
		// Handle MaybeInaccessibleMessage (zero value check)
		var msg *models.Message
		if update.CallbackQuery.Message != (models.MaybeInaccessibleMessage{}) && update.CallbackQuery.Message.Message != nil {
			msg = update.CallbackQuery.Message.Message
		} else {
			log.Printf("[ERROR] CallbackQuery does not have accessible message")
			return
		}

		parts := strings.SplitN(data, ":", 2)
		if len(parts) == 2 {
			action := parts[0]
			postID, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				log.Printf("[ERROR] Invalid postID in callback data: %v", err)
				return
			}

			if action == "approve" {
				log.Printf("[INFO] Admin %d approved post %d via inline button", userID, postID)
				err := ApprovePost(db, msg, approvedGroupID, postID)
				if err != nil {
					log.Printf("[ERROR] Failed to approve post %d: %v", postID, err)
				}
				return
			} else if action == "reject" {
				log.Printf("[INFO] Admin %d rejected post %d via inline button", userID, postID)
				err := RejectPost(db, msg, "Rejected by admin", postID)
				if err != nil {
					log.Printf("[ERROR] Failed to reject post %d: %v", postID, err)
				}
				return
			}
		}

		if data == "done" {
			// User clicked Done button in photo state; treat as if they sent "done"
			chatID := msg.Chat.ID
			messageID := msg.ID

			// Create a new update object that simulates a text message
			simulatedUpdate := models.Update{
				Message: &models.Message{
					ID:   messageID,
					Chat: msg.Chat,
					From: &update.CallbackQuery.From,
					Text: "done",
				},
			}

			username := ""
			if update.CallbackQuery.From.Username != "" {
				username = update.CallbackQuery.From.Username
			}
			// Use moderation group/topic for moderation post
			moderationGroupID := os.Getenv("MODERATION_GROUP_ID")
			moderationTopicID := os.Getenv("MODERATION_TOPIC_ID")
			mgid, _ := strconv.ParseInt(moderationGroupID, 10, 64)
			mtid, _ := strconv.Atoi(moderationTopicID)
			resp := HandleMessageWithDB(db, simulatedUpdate, globalBotInstance, mgid, lang, username, strconv.Itoa(mtid))
			// Remove the inline keyboard after click (edit message)
			editMarkup := &telegram.EditMessageReplyMarkupParams{
				ChatID:    chatID,
				MessageID: messageID,
			}
			_, _ = globalBotInstance.EditMessageReplyMarkup(context.Background(), editMarkup)
			// Send the preview or next step as a new message
			if resp != "" {
				msgParams := &telegram.SendMessageParams{
					ChatID:          chatID,
					Text:            resp,
					ReplyParameters: &models.ReplyParameters{MessageID: messageID},
				}
				_, err := globalBotInstance.SendMessage(context.Background(), msgParams)
				if err != nil {
					log.Printf("[ERROR] Failed to send preview after Done: %v", err)
				}
			}
			return
		}
		// Unknown callback data: ignore
		return
	}
}

// Add these helpers at the end of the file:

func isSafeUsername(username string) bool {
	for _, r := range username {
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// EscapeMarkdown escapes Telegram markdown special characters for safe output
func EscapeMarkdown(s string) string {
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
	)
	return replacer.Replace(s)
}

func GetHelpMessage(userID int64) string {
	helpMessage := "Available commands:\n" +
		"/start - Start a new sale post\n" +
		"/help - Show this help message"

	if IsAdmin(userID) {
		helpMessage += "\n\nAdmin commands:\n" +
			"/config <key> <value> - Set a config value\n" +
			"/config - Show all config values\n" +
			"/pending - List pending posts\n" +
			"/showdb - Show all tables in the database\n" +
			"/cleardb - Clear all posts and photos from the database"
	}

	return helpMessage
}
