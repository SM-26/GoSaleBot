package db

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

// Post represents a sale post to be persisted.
type Post struct {
	ID          int64
	UserID      int64
	ChatID      sql.NullInt64
	MessageID   sql.NullInt64
	Status      string
	Title       string
	Description sql.NullString
	Price       sql.NullString
	Location    sql.NullString
	Photos      []string
}

func GetPost(db *sql.DB, id int64) (*Post, error) {
	row := db.QueryRow("SELECT id, user_id, chat_id, message_id, status, title, description, price, location FROM posts WHERE id = ?", id)
	post := &Post{}
	err := row.Scan(
		&post.ID,
		&post.UserID,
		&post.ChatID,
		&post.MessageID,
		&post.Status,
		&post.Title,
		&post.Description,
		&post.Price,
		&post.Location,
	)
	if err != nil {
		return nil, err
	}

	// now load photos
	rows, err := db.Query("SELECT file_id FROM photos WHERE post_id = ?", id)
	if err != nil {
		// If no photos, it's not a critical error for GetPost
		log.Printf("[WARNING] GetPost couldn't query photos for post %d: %v", id, err)
		return post, nil
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("[WARN] failed to close rows: %v", err)
		}
	}()
	for rows.Next() {
		var fileID string
		if err := rows.Scan(&fileID); err == nil {
			post.Photos = append(post.Photos, fileID)
		}
	}

	return post, nil
}

func SavePhotoToDB(db *sql.DB, postID int64, fileID string) error {
	stmt, err := db.Prepare(`INSERT INTO photos (post_id, file_id) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := stmt.Close(); closeErr != nil {
			log.Printf("[WARN] failed to close statement in SavePhotoToDB: %v", closeErr)
		}
	}()
	_, err = stmt.Exec(postID, fileID)
	return err
}

// SavePostToDB saves a Post struct into the database. Returns the new post ID.
func SavePostToDB(db *sql.DB, post Post) (int64, error) {
	stmt, err := db.Prepare(`INSERT INTO posts (user_id, chat_id, message_id, status, title, description, price, location, created_at, expires_at, moderation_message_id, moderation_photo_message_ids) VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now', '+24 hours'), ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			log.Printf("[WARN] failed to close statement: %v", err)
		}
	}()
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
