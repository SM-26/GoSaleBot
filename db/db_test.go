package db

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Create tables
	_, err = db.Exec(`CREATE TABLE posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		chat_id INTEGER,
		message_id INTEGER,
		status TEXT,
		title TEXT,
		description TEXT,
		price TEXT,
		location TEXT,
		created_at DATETIME,
		expires_at DATETIME,
		moderation_photo_message_ids TEXT,
		moderation_message_id INTEGER
	)`)
	if err != nil {
		t.Fatalf("Failed to create posts table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE photos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id INTEGER,
		file_id TEXT
	)`)
	if err != nil {
		t.Fatalf("Failed to create photos table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE config (
		key TEXT PRIMARY KEY,
		value TEXT
	)`)
	if err != nil {
		t.Fatalf("Failed to create config table: %v", err)
	}

	return db
}

func TestSetConfig(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	err := SetConfig(db, "test_key", "test_value")
	if err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	var value string
	err = db.QueryRow("SELECT value FROM config WHERE key = 'test_key'").Scan(&value)
	if err != nil {
		t.Fatalf("Failed to query config value: %v", err)
	}

	if value != "test_value" {
		t.Errorf("Expected value 'test_value', but got '%s'", value)
	}
}

func TestGetConfig(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec("INSERT INTO config (key, value) VALUES ('test_key', 'test_value')")
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	value, err := GetConfig(db, "test_key")
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	if value != "test_value" {
		t.Errorf("Expected value 'test_value', but got '%s'", value)
	}
}

func TestSavePhotoToDB(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	err := SavePhotoToDB(db, 1, "file123")
	if err != nil {
		t.Fatalf("SavePhotoToDB failed: %v", err)
	}

	var fileID string
	err = db.QueryRow("SELECT file_id FROM photos WHERE post_id = 1").Scan(&fileID)
	if err != nil {
		t.Fatalf("Failed to query photo: %v", err)
	}

	if fileID != "file123" {
		t.Errorf("Expected file_id 'file123', but got '%s'", fileID)
	}
}

func TestSavePostToDB(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	post := Post{
		UserID:      9876,
		ChatID:      int64(12345),
		MessageID:   54321,
		Title:       "Test Title",
		Description: "Test Description",
		Price:       "100",
		Location:    "Test Location",
		Photos:      []string{"file1", "file2"},
	}

	postID, err := SavePostToDB(db, post)
	if err != nil {
		t.Fatalf("SavePostToDB failed: %v", err)
	}

	if postID == 0 {
		t.Error("Expected a non-zero post ID")
	}

	var title string
	err = db.QueryRow("SELECT title FROM posts WHERE id = ?", postID).Scan(&title)
	if err != nil {
		t.Fatalf("Failed to query post: %v", err)
	}

	if title != "Test Title" {
		t.Errorf("Expected title 'Test Title', but got '%s'", title)
	}

	var photoCount int
	err = db.QueryRow("SELECT COUNT(*) FROM photos WHERE post_id = ?", postID).Scan(&photoCount)
	if err != nil {
		t.Fatalf("Failed to query photo count: %v", err)
	}

	if photoCount != 2 {
		t.Errorf("Expected 2 photos, but got %d", photoCount)
	}
}
