package db

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

// Post represents a sale post to be persisted.
type Post struct {
	UserID      int64
	ChatID      int64
	MessageID   int
	Title       string
	Description string
	Price       string
	Location    string
	Photos      []string
}

func SavePhotoToDB(db *sql.DB, postID int64, fileID string) error {
	stmt, err := db.Prepare(`INSERT INTO photos (post_id, file_id) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(postID, fileID)
	return err
}

// SavePostToDB saves a Post struct into the database. Returns the new post ID.
func SavePostToDB(db *sql.DB, post Post) (int64, error) {
	stmt, err := db.Prepare(`INSERT INTO posts (user_id, chat_id, message_id, status, title, description, price, location, created_at, expires_at, moderation_message_id, moderation_photo_message_ids) VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now', '+24 hours'), ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	res, err := stmt.Exec(
		post.UserID,
		post.ChatID,
		post.MessageID,
		"pending",
		post.Title,
		post.Description,
		post.Price,
		post.Location,
		0,
		"",
	)
	if err != nil {
		return 0, err
	}
	postID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if len(post.Photos) > 0 {
		for _, fileID := range post.Photos {
			if err := SavePhotoToDB(db, postID, fileID); err != nil {
				log.Printf("[ERROR] SavePhotoToDB in SavePostToDB: %v", err)
			}
		}
	}
	return postID, nil
}

// Config table helpers
func GetConfig(db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	return value, err
}

func SetConfig(db *sql.DB, key, value string) error {
	_, err := db.Exec(`INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}
