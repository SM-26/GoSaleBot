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
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var id, userIDVal, chatID, messageID int64
				var status, title, price, location, createdAt, expiresAt string
				var description sql.NullString
				err := rows.Scan(&id, &userIDVal, &chatID, &messageID, &status, &title, &description, &price, &location, &createdAt, &expiresAt)
				if err != nil {
					continue
				}
				descStr := ""
				if description.Valid {
					descStr = description.String
				}
				fmt.Fprintf(&out, "ID: %d, User: %d, Chat: %d, Msg: %d, Status: %s, Title: %s, Desc: %s, Price: %s, Loc: %s, Created: %s, Expires: %s\n",
					id, userIDVal, chatID, messageID, status, title, descStr, price, location, createdAt, expiresAt)
			}
		}
		out.WriteString("---" + "Users" + "---" + "\n")
		userRows, err := dbConn.Query("SELECT id, username FROM users")
		if err != nil {
			log.Printf("[ERROR] Failed to query users: %v", err)
			out.WriteString("Failed to query users: " + err.Error() + "\n")
		} else {
			defer func() { _ = userRows.Close() }()
			for userRows.Next() {
				var id int64
				var uname string
				if err := userRows.Scan(&id, &uname); err == nil {
					fmt.Fprintf(&out, "ID: %d, Username: %s\n", id, uname)
				}
			}
		}
		out.WriteString("---" + "Photos" + "---" + "\n")
		photoRows, err := dbConn.Query("SELECT id, post_id, file_id FROM photos")
		if err != nil {
			log.Printf("[ERROR] Failed to query photos: %v", err)
			out.WriteString("Failed to query photos: " + err.Error() + "\n")
		} else {
			defer func() { _ = photoRows.Close() }()
			for photoRows.Next() {
				var id, postID int64
				var fileID string
				_ = photoRows.Scan(&id, &postID, &fileID)
				fmt.Fprintf(&out, "ID: %d, PostID: %d, FileID: %s\n", id, postID, fileID)
			}
		}
		out.WriteString("---" + "Config" + "---" + "\n")
		configRows, err := dbConn.Query("SELECT key, value FROM config")
		if err != nil {
			log.Printf("[ERROR] Failed to query config: %v", err)
			out.WriteString("Failed to query config: " + err.Error() + "\n")
		} else {
			defer func() { _ = configRows.Close() }()
			for configRows.Next() {
				var key, value string
				_ = configRows.Scan(&key, &value)
				fmt.Fprintf(&out, "%s = %s\n", key, value)
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
		var out strings.Builder
		rows, err := dbConn.Query("SELECT key, value FROM config ORDER BY key")
		if err == nil {
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var key, val string
				if err := rows.Scan(&key, &val); err != nil {
					log.Printf("[ERROR] Failed to scan config row: %v", err)
					continue
				}
				fmt.Fprintf(&out, "%s = %s\n", key, val)
			}
			if err := rows.Err(); err != nil {
				log.Printf("[ERROR] Rows error: %v", err)
			}
		} else {
			fmt.Fprintf(&out, "Failed to query config: %v\n", err)
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
					return i18n.T(lang, "invalid_post_id")
				}

				post, err := db.GetPost(dbConn, postID)
				if err != nil {
					if err == sql.ErrNoRows {
						return i18n.T(lang, "no_pending_post_found")
					}
					log.Printf("[ERROR] /pending approve: failed to get post %d: %v", postID, err)
					return i18n.T(lang, "failed_fetch_post_details")
				}

				if post.Status != StatusPending {
					return i18n.T(lang, "no_pending_post_found")
				}

				if parts[2] == "approve" {
					approvedGroupID, _ := strconv.ParseInt(os.Getenv("APPROVED_GROUP_ID"), 10, 64)
					approvedTopicIDStr, _ := db.GetConfig(dbConn, "APPROVED_TOPIC_ID")
					approvedTopicID, _ := strconv.Atoi(approvedTopicIDStr)

					_, err = dbConn.Exec("UPDATE posts SET status = ? WHERE id = ?", StatusApproved, postID)
					if err != nil {
						return i18n.T(lang, "failed_update_config")
					}

					var username string
					row := dbConn.QueryRow("SELECT username FROM users WHERE id = ?", post.UserID)
					_ = row.Scan(&username)
					postedBy := formatPostedBy(username, post.UserID)

					title := escapeHTML(post.Title)
					var description string
					if post.Description.Valid {
						description = escapeHTML(post.Description.String)
					}
					price := escapeHTML(post.Price.String)
					location := escapeHTML(post.Location.String)

					msgText := i18n.T(lang, "for_sale", title, description, price, location, postedBy)

					photoFileIDs := post.Photos
					ctx := context.Background()
					if len(photoFileIDs) > 0 {
						var mediaGroup []models.InputMedia
						for i, fileID := range photoFileIDs {
							media := &models.InputMediaPhoto{Media: fileID}
							if i == 0 {
								media.Caption = msgText
								media.ParseMode = "HTML"
							}
							mediaGroup = append(mediaGroup, media)
						}
						mediaReq := &telegram.SendMediaGroupParams{ChatID: approvedGroupID, Media: mediaGroup}
						if approvedTopicID != 0 {
							mediaReq.MessageThreadID = approvedTopicID
						}
						if globalBotInstance != nil {
							_, err = globalBotInstance.SendMediaGroup(ctx, mediaReq)
							if err != nil {
								return i18n.T(lang, "failed_send_approved_media")
							}
						}
					} else {
						if globalBotInstance != nil {
							msgReq := &telegram.SendMessageParams{ChatID: approvedGroupID, Text: msgText, ParseMode: "HTML"}
							if approvedTopicID != 0 {
								msgReq.MessageThreadID = approvedTopicID
							}
							_, err = globalBotInstance.SendMessage(ctx, msgReq)
							if err != nil {
								return i18n.T(lang, "failed_send_approved")
							}
						}
					}
					if globalBotInstance != nil {
						notifyText := i18n.T(lang, "post_approved")
						notifyReq := &telegram.SendMessageParams{ChatID: post.UserID, Text: notifyText}
						_, _ = globalBotInstance.SendMessage(ctx, notifyReq)
					}
					return i18n.T(lang, "post_approved_published")
				} else { // reject
					_, err = dbConn.Exec("UPDATE posts SET status = ? WHERE id = ?", StatusRejected, postID)
					if err != nil {
						return i18n.T(lang, "failed_update_config")
					}
					if globalBotInstance != nil {
						notifyText := i18n.T(lang, "post_rejected", "Rejected by admin")
						notifyReq := &telegram.SendMessageParams{ChatID: post.UserID, Text: notifyText}
						_, _ = globalBotInstance.SendMessage(context.Background(), notifyReq)
					}
					return i18n.T(lang, "post_rejected")
				}
			}
		}
		// list pending
		// list pending
		rows, err := dbConn.Query("SELECT id, user_id, title, created_at FROM posts WHERE status = ?", StatusPending)
		if err != nil {
			log.Printf("[ERROR] Failed to query pending posts: %v", err)
			return i18n.T(lang, "failed_query_pending", err.Error())
		}
		defer func() { _ = rows.Close() }()
		var out strings.Builder
		for rows.Next() {
			var id, userIDVal int64
			var title, createdAt string
			if err := rows.Scan(&id, &userIDVal, &title, &createdAt); err != nil {
				log.Printf("[ERROR] Failed to scan pending post: %v", err)
				continue
			}
			fmt.Fprintf(&out, "ID: %d, User: %d, Title: %s, Created: %s\n", id, userIDVal, title, createdAt)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[ERROR] Rows iteration error: %v", err)
		}
		log.Printf("[INFO] Admin %d listed pending posts", userID)
		return out.String()
	}

	return handlers
}
