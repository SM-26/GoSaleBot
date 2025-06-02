package db

import (
	"database/sql"
	"log"
	_ "github.com/mattn/go-sqlite3"
)

func SavePhotoToDB(db *sql.DB, postID int64, fileID string) error {
	stmt, err := db.Prepare(`INSERT INTO photos (post_id, file_id) VALUES (?, ?)`)
	if err != nil {
		log.Printf("[ERROR] Prepare SavePhotoToDB: %v", err)
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(postID, fileID)
	if err != nil {
		log.Printf("[ERROR] Exec SavePhotoToDB: %v", err)
	}
	return err
}

func SavePostToDB(db *sql.DB, userID int64, postData map[string]interface{}) (int64, error) {
	stmt, err := db.Prepare(`INSERT INTO posts (user_id, chat_id, message_id, status, title, description, price, location, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now', '+24 hours'))`)
	if err != nil {
		log.Printf("[ERROR] Prepare SavePostToDB: %v", err)
		return 0, err
	}
	defer stmt.Close()
	chatID, _ := postData["chat_id"].(int64)
	messageID, _ := postData["message_id"].(int)
	res, err := stmt.Exec(
		userID,
		chatID,
		messageID,
		"pending",
		postData["title"],
		postData["description"],
		postData["price"],
		postData["location"],
	)
	if err != nil {
		log.Printf("[ERROR] Exec SavePostToDB: %v", err)
		return 0, err
	}
	postID, err := res.LastInsertId()
	if err != nil {
		log.Printf("[ERROR] LastInsertId SavePostToDB: %v", err)
		return 0, err
	}
	if photos, ok := postData["photos"].([]string); ok {
		for _, fileID := range photos {
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
