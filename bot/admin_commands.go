package bot

import (
	"context"
	"database/sql"
	"fmt"
	"gosalebot/db"
	"gosalebot/i18n"
	"log"
	"os"
	"strconv"
	"strings"

	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// AdminHandlers returns a map of admin command handlers. Each handler accepts the full command text and returns a response string.
func AdminHandlers(dbConn *sql.DB, userID int64, lang string) map[string]func(string) string {
	handlers := map[string]func(string) string{}

	handlers["/showdb"] = func(_ string) string {
		var out strings.Builder
		out.WriteString("---" + "Posts" + "---" + "\n")

		rows, err := dbConn.Query("SELECT id, user_id, chat_id, message_id, status, title, description, price, location, created_at, expires_at FROM posts")
		if err != nil {
			log.Printf("[ERROR] Failed to query posts: %v", err)
			out.WriteString("Failed to query posts: " + err.Error() + "\n")
		} else {
			defer rows.Close()
			for rows.Next() {
				var id, userIDVal, chatID, messageID int64
				var status, title, description, price, location, createdAt, expiresAt string
				_ = rows.Scan(&id, &userIDVal, &chatID, &messageID, &status, &title, &description, &price, &location, &createdAt, &expiresAt)
				out.WriteString(fmt.Sprintf("ID: %d, User: %d, Chat: %d, Msg: %d, Status: %s, Title: %s, Desc: %s, Price: %s, Loc: %s, Created: %s, Expires: %s\n",
					id, userIDVal, chatID, messageID, status, title, description, price, location, createdAt, expiresAt))
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
				var uname string
				_ = userRows.Scan(&id, &uname)
				out.WriteString(fmt.Sprintf("ID: %d, Username: %s\n", id, uname))
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

	handlers["/cleardb"] = func(_ string) string {
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

	handlers["/config"] = func(full string) string {
		if strings.HasPrefix(full, "/config ") {
			parts := strings.SplitN(full, " ", 3)
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
		// list config
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

	handlers["/pending"] = func(full string) string {
		// /pending or /pending <id> approve|reject
		if strings.HasPrefix(full, "/pending ") {
			parts := strings.Fields(full)
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

				if parts[2] == "approve" {
					approvedGroupID, _ := strconv.ParseInt(os.Getenv("APPROVED_GROUP_ID"), 10, 64)
					row = dbConn.QueryRow("SELECT description, price, location FROM posts WHERE id = ?", postID)
					var description, price, location string
					err = row.Scan(&description, &price, &location)
					if err != nil {
						return "Failed to fetch post details."
					}
					_, err = dbConn.Exec("UPDATE posts SET status = ? WHERE id = ?", StatusApproved, postID)
					if err != nil {
						return "Failed to update post status."
					}
					var username string
					row = dbConn.QueryRow("SELECT username FROM users WHERE id = ?", userIDVal)
					_ = row.Scan(&username)
					postedBy := formatPostedBy(username, userIDVal)
					msgText := telegram.EscapeMarkdown(i18n.T(lang, "for_sale", title, description, price, location, postedBy))
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
							media := &models.InputMediaPhoto{Media: fileID}
							if i == 0 {
								media.Caption = msgText
								media.ParseMode = "Markdown"
							}
							mediaGroup = append(mediaGroup, media)
						}
						mediaReq := &telegram.SendMediaGroupParams{ChatID: approvedGroupID, Media: mediaGroup}
						if globalBotInstance != nil {
							_, err = globalBotInstance.SendMediaGroup(ctx, mediaReq)
							if err != nil {
								return "Failed to send approved post (media group)."
							}
						}
					} else {
						if globalBotInstance != nil {
							msgReq := &telegram.SendMessageParams{ChatID: approvedGroupID, Text: msgText, ParseMode: "Markdown"}
							_, err = globalBotInstance.SendMessage(ctx, msgReq)
							if err != nil {
								return "Failed to send approved post."
							}
						}
					}
					if globalBotInstance != nil {
						notifyText := i18n.T(lang, "post_approved")
						notifyReq := &telegram.SendMessageParams{ChatID: userIDVal, Text: notifyText}
						_, _ = globalBotInstance.SendMessage(ctx, notifyReq)
					}
					return "Post approved and published."
				} else {
					_, err = dbConn.Exec("UPDATE posts SET status = ? WHERE id = ?", StatusRejected, postID)
					if err != nil {
						return "Failed to update post status."
					}
					if globalBotInstance != nil {
						notifyText := i18n.T(lang, "post_rejected", "Rejected by admin")
						notifyReq := &telegram.SendMessageParams{ChatID: userIDVal, Text: notifyText}
						_, _ = globalBotInstance.SendMessage(context.Background(), notifyReq)
					}
					return "Post rejected."
				}
			}
		}
		// list pending
		rows, err := dbConn.Query("SELECT id, user_id, title, created_at FROM posts WHERE status = ?", StatusPending)
		if err != nil {
			log.Printf("[ERROR] Failed to query pending posts: %v", err)
			return "Failed to query pending posts: " + err.Error()
		}
		defer rows.Close()
		var out strings.Builder
		for rows.Next() {
			var id, userIDVal int64
			var title, createdAt string
			_ = rows.Scan(&id, &userIDVal, &title, &createdAt)
			out.WriteString(fmt.Sprintf("ID: %d, User: %d, Title: %s, Created: %s\n", id, userIDVal, title, createdAt))
		}
		log.Printf("[INFO] Admin %d listed pending posts", userID)
		return out.String()
	}

	return handlers
}
