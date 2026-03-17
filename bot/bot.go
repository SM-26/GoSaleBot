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

// escapeMarkdown escapes Markdown special characters including [ and ] for safe use in Telegram Markdown
func escapeMarkdown(text string) string {
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, "*", "\\*")
	text = strings.ReplaceAll(text, "_", "\\_")
	text = strings.ReplaceAll(text, "`", "\\`")
	text = strings.ReplaceAll(text, "[", "\\[")
	text = strings.ReplaceAll(text, "]", "\\]")
	return text
}

// escapeHTML escapes HTML special characters for safe use in Telegram HTML
func escapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

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
	if update.Message == nil {
		return ""
	}

	// Handle admin rejection via reply
	if update.Message.ReplyToMessage != nil && IsAdmin(update.Message.From.ID) {
		if response := handleRejectViaReply(dbConn, update.Message, moderationGroupID); response != "" {
			return response
		}
	}

	// Prepare shared handler context (session, username, topic)
	saveUsername, moderationTopicID, session := prepareHandlerContext(update.Message, username...)

	// State machine logic
	switch session.State {
	case fsm.StateIdle:
		return handleIdleState(dbConn, session, update.Message, lang, saveUsername)
	case fsm.StateTitle:
		res := HandleTitleState(session, update.Message, lang)
		return ExecuteHandlerResult(dbConn, bot, session, res, moderationGroupID, moderationTopicID, lang, saveUsername)
	case fsm.StateDescription:
		res := HandleDescriptionState(session, update.Message, lang)
		return ExecuteHandlerResult(dbConn, bot, session, res, moderationGroupID, moderationTopicID, lang, saveUsername)
	case fsm.StatePrice:
		// Validate price according to config before accepting and moving on
		priceText := update.Message.Text
		if ok, msg := ValidatePrice(dbConn, priceText, lang); !ok {
			// Do not change state; send validation message
			return msg
		}
		res := HandlePriceState(session, update.Message, lang)
		return ExecuteHandlerResult(dbConn, bot, session, res, moderationGroupID, moderationTopicID, lang, saveUsername)
	case fsm.StateLocation:
		res := HandleLocationState(session, update.Message, lang)
		return ExecuteHandlerResult(dbConn, bot, session, res, moderationGroupID, moderationTopicID, lang, saveUsername)
	case fsm.StatePhotos:
		res := HandlePhotosStatePure(session, update, lang, saveUsername)
		// If handler intends to SavePost, validate photos before persisting
		if res.SavePost != nil {
			// Determine photos list from Draft
			var photos []string
			if session.Draft != nil {
				photos = session.Draft.Photos
			}
			if ok, msg := ValidatePhotos(dbConn, photos, lang); !ok {
				// Return validation message and do not save/clear state
				return msg
			}
		}
		return ExecuteHandlerResult(dbConn, bot, session, res, moderationGroupID, moderationTopicID, lang, saveUsername)
	default:
		log.Printf("[WARNING] Session reset for user %d due to unknown state", session.UserID)
		session.State = fsm.StateIdle
		return i18n.T(lang, "session_reset")
	}
}

func handleRejectViaReply(dbConn *sql.DB, msg *models.Message, moderationGroupID int64) string {
	repliedToMsgID := msg.ReplyToMessage.ID
	repliedToChatID := msg.ReplyToMessage.Chat.ID

	if repliedToChatID == moderationGroupID {
		var postID int64
		row := dbConn.QueryRow("SELECT id FROM posts WHERE moderation_message_id = ?", repliedToMsgID)
		err := row.Scan(&postID)
		if err == nil {
			log.Printf("[INFO] Admin %d rejected post %d via reply with reason: %s", msg.From.ID, postID, msg.Text)
			err := RejectPost(dbConn, msg.ReplyToMessage, msg.Text, postID)
			if err != nil {
				log.Printf("[ERROR] Failed to reject post %d via reply: %v", postID, err)
				lang := os.Getenv("LANG")
				if lang == "" {
					lang = "en"
				}
				return i18n.T(lang, "failed_reject_post")
			}
			lang := os.Getenv("LANG")
			if lang == "" {
				lang = "en"
			}
			return i18n.T(lang, "post_rejected_custom")
		}
	}
	return ""
}

// prepareHandlerContext centralizes session retrieval and username/topic extraction.
func prepareHandlerContext(msg *models.Message, username ...string) (saveUsername string, moderationTopicID int, session *fsm.UserSession) {
	userID := msg.From.ID
	session, ok := fsm.Sessions[userID]
	if !ok {
		session = &fsm.UserSession{UserID: userID, State: fsm.StateIdle}
		fsm.Sessions[userID] = session
		log.Printf("[INFO] New session created for user %d", userID)
	}

	// Determine username for saving
	if len(username) > 0 && username[0] != "" {
		saveUsername = username[0]
	} else if msg.From != nil {
		saveUsername = strings.TrimSpace(msg.From.FirstName + " " + msg.From.LastName)
	}

	// Extract optional moderation topic ID
	moderationTopicID = 0
	if len(username) > 1 {
		moderationTopicID, _ = strconv.Atoi(username[1])
	}
	return
}

func handleIdleState(dbConn *sql.DB, session *fsm.UserSession, msg *models.Message, lang, saveUsername string) string {
	if msg.Text == "/start" {
		_, err := dbConn.Exec(`INSERT OR IGNORE INTO users (id, username) VALUES (?, ?)`, session.UserID, saveUsername)
		if err != nil {
			log.Printf("[ERROR] Failed to insert user %d: %v", session.UserID, err)
		} else {
			log.Printf("[INFO] User %d started bot. Username: %s", session.UserID, saveUsername)
		}
		session.State = fsm.StateTitle
		return i18n.T(lang, "welcome")
	}
	// Handle /myposts and subcommands
	if strings.HasPrefix(msg.Text, "/myposts") {
		parts := strings.Fields(msg.Text)
		if len(parts) == 1 {
			return HandleMyPosts(dbConn, session.UserID, lang)
		}
		if len(parts) == 3 {
			// /myposts <id> delete|sold
			idStr := parts[1]
			action := parts[2]
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				return i18n.T(lang, "invalid_myposts_usage")
			}
			switch action {
			case "delete":
				ok, msgText := deletePost(dbConn, session.UserID, id, lang)
				_ = ok
				return msgText
			case "sold", "mark_sold":
				ok, msgText := markPostSold(dbConn, session.UserID, id, lang)
				_ = ok
				return msgText
			}
			return i18n.T(lang, "invalid_myposts_usage")
		}
		return i18n.T(lang, "invalid_myposts_usage")
	}
	if msg.Chat.ID > 0 {
		log.Printf("[INFO] User %d prompted with /start in private chat", session.UserID)
		return i18n.T(lang, "start")
	}
	log.Printf("[WARNING] Ignored message from user %d in group/channel", session.UserID)
	return ""
}

// ... handlers refactored into dedicated files (handlers_text.go, handlers_price.go, handlers_location.go, handlers_photos.go)

func savePost(dbConn *sql.DB, session *fsm.UserSession, chatID, messageID int64) (int64, error) {
	var post db.Post
	// Build post from typed Draft
	if session.Draft == nil {
		session.Draft = &fsm.PostDraft{}
	}
	post = db.Post{
		UserID:      session.UserID,
		ChatID:      chatID,
		MessageID:   int(messageID),
		Title:       session.Draft.Title,
		Description: session.Draft.Description,
		Price:       session.Draft.Price,
		Location:    session.Draft.Location,
		Photos:      session.Draft.Photos,
	}
	postID, err := db.SavePostToDB(dbConn, post)
	if err != nil {
		log.Printf("[ERROR] Failed to save post: %v", err)
		return 0, err
	}
	log.Printf("[INFO] Post submitted by user %d (postID: %d) for moderation", session.UserID, postID)
	return postID, nil
}

func sendModerationMessage(dbConn *sql.DB, bot *telegram.Bot, session *fsm.UserSession, postID, moderationGroupID int64, moderationTopicID int, lang, saveUsername string) {
	var title interface{}
	var description interface{}
	var price interface{}
	var location interface{}
	var photos []string
	// Use typed Draft for moderation message
	if session.Draft != nil {
		title = session.Draft.Title
		description = session.Draft.Description
		price = session.Draft.Price
		location = session.Draft.Location
		photos = session.Draft.Photos
	}

	postedBy := formatPostedBy(saveUsername, session.UserID)
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
		sendModerationMessageWithPhotos(ctx, dbConn, bot, postID, moderationGroupID, moderationTopicID, moderationMsg, replyMarkup, photos)
	} else {
		sendModerationMessageOnly(ctx, dbConn, bot, postID, moderationGroupID, moderationTopicID, moderationMsg, replyMarkup)
	}
}

func sendModerationMessageWithPhotos(ctx context.Context, dbConn *sql.DB, bot *telegram.Bot, postID, moderationGroupID int64, moderationTopicID int, moderationMsg string, replyMarkup *models.InlineKeyboardMarkup, photos []string) {
	if bot == nil {
		log.Printf("[INFO] Bot is nil, skipping sendModerationMessageWithPhotos for post %d", postID)
		return
	}

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
		updateModerationMessageID(dbConn, postID, int64(sentModMsg.ID))
	}

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
		updateModerationPhotoMessageIDs(dbConn, postID, moderationPhotoMessageIDs)
	}
}

func sendModerationMessageOnly(ctx context.Context, dbConn *sql.DB, bot *telegram.Bot, postID, moderationGroupID int64, moderationTopicID int, moderationMsg string, replyMarkup *models.InlineKeyboardMarkup) {
	if bot == nil {
		log.Printf("[INFO] Bot is nil, skipping sendModerationMessageOnly for post %d", postID)
		return
	}

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
		updateModerationMessageID(dbConn, postID, int64(sentModMsg.ID))
	}
}

func updateModerationMessageID(dbConn *sql.DB, postID int64, messageID int64) {
	_, err := dbConn.Exec("UPDATE posts SET moderation_message_id = ? WHERE id = ?", messageID, postID)
	if err != nil {
		log.Printf("[ERROR] Failed to update moderation_message_id: %v", err)
	}
}

func updateModerationPhotoMessageIDs(dbConn *sql.DB, postID int64, messageIDs []string) {
	_, err := dbConn.Exec("UPDATE posts SET moderation_photo_message_ids = ? WHERE id = ?", strings.Join(messageIDs, ","), postID)
	if err != nil {
		log.Printf("[ERROR] Failed to update moderation_photo_message_ids: %v", err)
	}
}

// ApprovePost publishes a post to the approved group and deletes the moderation message.
func ApprovePost(dbConn *sql.DB, moderationMsg *models.Message, approvedGroupID int64, approvedTopicID int, postID int64) error {
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
	title = escapeHTML(title)
	description = escapeHTML(description)
	price = escapeHTML(price)
	location = escapeHTML(location)
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
				media.ParseMode = "HTML"
			}
			mediaGroup = append(mediaGroup, media)
		}
		mediaReq := &telegram.SendMediaGroupParams{
			ChatID: approvedGroupID,
			Media:  mediaGroup,
		}
		if approvedTopicID != 0 {
			mediaReq.MessageThreadID = approvedTopicID
		}
		_, err = globalBotInstance.SendMediaGroup(ctx, mediaReq)
		if err != nil {
			log.Printf("[ERROR] ApprovePost: failed to send media group: %v", err)
		}
	} else {
		msgReq := &telegram.SendMessageParams{
			ChatID:    approvedGroupID,
			Text:      msgText,
			ParseMode: "HTML",
		}
		if approvedTopicID != 0 {
			msgReq.MessageThreadID = approvedTopicID
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
	if moderationPhotoMessageIDsStr.Valid && moderationPhotoMessageIDsStr.String != "" {
		moderationImageMsgIDs := strings.Split(moderationPhotoMessageIDsStr.String, ",")
		for _, msgIDStr := range moderationImageMsgIDs {
			if msgIDStr == "" {
				continue
			}
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
			return "@" + escapeHTML(username)
		}
		return escapeHTML(username)
	}
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = "en"
	}
	return fmt.Sprintf("<a href=\"tg://user?id=%d\">%s</a>", userID, i18n.T(lang, "user_link_text"))
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
	if globalBotInstance != nil {
		_, sendErr := globalBotInstance.SendMessage(ctx, msgReq)
		if sendErr != nil {
			log.Printf("[WARNING] RejectPost: failed to notify user: %v", sendErr)
		}
	} else {
		log.Printf("[INFO] globalBotInstance is nil, skipping notify user for rejected post %d", postID)
	}
	// Delete moderation message
	deleteReq := &telegram.DeleteMessageParams{
		ChatID:    moderationMsg.Chat.ID,
		MessageID: moderationMsg.ID,
	}
	if globalBotInstance != nil {
		_, delErr := globalBotInstance.DeleteMessage(ctx, deleteReq)
		if delErr != nil {
			log.Printf("[WARNING] RejectPost: failed to delete moderation message: %v", delErr)
		}
	} else {
		log.Printf("[INFO] globalBotInstance is nil, skipping delete of moderation message for post %d", postID)
	}
	log.Printf("[INFO] Post %d rejected by admin", postID)
	return nil
}

// deletePost deletes a post if it belongs to the requesting user.
func deletePost(dbConn *sql.DB, requestingUserID, postID int64, lang string) (bool, string) {
	// verify ownership
	var ownerID int64
	row := dbConn.QueryRow("SELECT user_id FROM posts WHERE id = ?", postID)
	if err := row.Scan(&ownerID); err != nil {
		return false, i18n.T(lang, "post_not_found")
	}
	if ownerID != requestingUserID {
		return false, i18n.T(lang, "not_your_post")
	}
	// delete photos and post
	if _, err := dbConn.Exec("DELETE FROM photos WHERE post_id = ?", postID); err != nil {
		// log and continue
		log.Printf("[ERROR] Failed to delete photos for post %d: %v", postID, err)
	}
	if _, err := dbConn.Exec("DELETE FROM posts WHERE id = ?", postID); err != nil {
		log.Printf("[ERROR] Failed to delete post %d: %v", postID, err)
		return false, i18n.T(lang, "post_not_found")
	}
	return true, i18n.T(lang, "post_deleted", postID)
}

// markPostSold marks a post as sold if it belongs to the requesting user.
func markPostSold(dbConn *sql.DB, requestingUserID, postID int64, lang string) (bool, string) {
	var ownerID int64
	row := dbConn.QueryRow("SELECT user_id FROM posts WHERE id = ?", postID)
	if err := row.Scan(&ownerID); err != nil {
		return false, i18n.T(lang, "post_not_found")
	}
	if ownerID != requestingUserID {
		return false, i18n.T(lang, "not_your_post")
	}
	// set status to 'sold' (create if not exists)
	if _, err := dbConn.Exec("UPDATE posts SET status = ? WHERE id = ?", "sold", postID); err != nil {
		log.Printf("[ERROR] Failed to mark post %d as sold: %v", postID, err)
		return false, i18n.T(lang, "post_not_found")
	}
	return true, i18n.T(lang, "post_marked_sold", postID)
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
	handlers := AdminHandlers(dbConn, userID, lang)

	// Dispatch based on first token
	base := text
	if strings.Contains(text, " ") {
		base = strings.SplitN(text, " ", 2)[0]
	}
	if handler, ok := handlers[base]; ok {
		return handler(text)
	}
	log.Printf("[WARNING] Unknown admin command by user %d: %s", userID, text)
	return i18n.T(lang, "unknown_admin_command")

}

func HandleCallbackQuery(db *sql.DB, update models.Update, botAPI *telegram.Bot, approvedGroupID int64, approvedTopicID int) {
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

			switch action {
			case "approve":
				log.Printf("[INFO] Admin %d approved post %d via inline button", userID, postID)
				err := ApprovePost(db, msg, approvedGroupID, approvedTopicID, postID)
				if err != nil {
					log.Printf("[ERROR] Failed to approve post %d: %v", postID, err)
				}
				return
			case "reject":
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

func GetHelpMessage(userID int64) string {
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = "en"
	}

	helpMessage := i18n.T(lang, "help_available_commands") + "\n" +
		i18n.T(lang, "help_cmd_start") + "\n" +
		i18n.T(lang, "help_cmd_help")

	if IsAdmin(userID) {
		helpMessage += "\n\n" + i18n.T(lang, "help_admin_commands") + "\n" +
			i18n.T(lang, "help_cmd_config_set") + "\n" +
			i18n.T(lang, "help_cmd_config_list") + "\n" +
			i18n.T(lang, "help_cmd_pending") + "\n" +
			i18n.T(lang, "help_cmd_showdb") + "\n" +
			i18n.T(lang, "help_cmd_cleardb")
	}

	helpMessage += "\n\n" + i18n.T(lang, "help_user_commands") + "\n" +
		i18n.T(lang, "help_cmd_myposts")

	return helpMessage
}

// HandleMyPosts returns a human-readable list of posts for the requesting user.
func HandleMyPosts(dbConn *sql.DB, userID int64, lang string) string {
	rows, err := dbConn.Query("SELECT id, title, status, created_at FROM posts WHERE user_id = ? ORDER BY created_at DESC", userID)
	if err != nil {
		return i18n.T(lang, "failed_clear_posts", err.Error())
	}
	defer rows.Close()
	var out strings.Builder
	out.WriteString(i18n.T(lang, "your_posts_header"))
	found := false
	for rows.Next() {
		var id int64
		var title, status, created string
		_ = rows.Scan(&id, &title, &status, &created)
		if title == "" {
			title = "<no title>"
		}
		// truncate title to 40 chars
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		out.WriteString(i18n.T(lang, "post_line", id, title, status, created))
		found = true
	}
	if !found {
		return i18n.T(lang, "no_posts")
	}
	return out.String()
}
