package bot

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func newAdminTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}
	_, err = db.Exec(`
        CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT);
        CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER, chat_id INTEGER, message_id INTEGER, status TEXT, title TEXT, description TEXT, price TEXT, location TEXT, created_at TEXT, expires_at TEXT, moderation_message_id INTEGER, moderation_photo_message_ids TEXT);
        CREATE TABLE photos (id INTEGER PRIMARY KEY, post_id INTEGER, file_id TEXT);
        CREATE TABLE config (key TEXT PRIMARY KEY, value TEXT);
    `)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}
	return db
}

func TestAdminShowAndClearDB(t *testing.T) {
	db := newAdminTestDB(t)
	defer db.Close()

	// insert sample data
	_, _ = db.Exec(`INSERT INTO users (id, username) VALUES (1, 'u1')`)
	_, _ = db.Exec(`INSERT INTO posts (id, user_id, status, title) VALUES (1, 1, 'pending', 'T1')`)
	_, _ = db.Exec(`INSERT INTO photos (id, post_id, file_id) VALUES (1, 1, 'f1')`)
	_, _ = db.Exec(`INSERT INTO config (key, value) VALUES ('K','V')`)

	os.Setenv("ADMINS", "42")
	LoadAdminsFromEnv()

	// showdb
	resp := HandleAdminCommand(db, 42, "/showdb", "admin")
	if resp == "" {
		t.Fatalf("Expected non-empty showdb output")
	}

	// cleardb
	resp = HandleAdminCommand(db, 42, "/cleardb", "admin")
	if resp == "" {
		t.Fatalf("Expected response from cleardb")
	}
	var cnt int
	_ = db.QueryRow("SELECT COUNT(*) FROM posts").Scan(&cnt)
	if cnt != 0 {
		t.Fatalf("Expected 0 posts after cleardb, got %d", cnt)
	}
}

func TestAdminPendingApproveReject(t *testing.T) {
	db := newAdminTestDB(t)
	defer db.Close()

	// insert a pending post
	_, _ = db.Exec(`INSERT INTO users (id, username) VALUES (10, 'u10')`)
	_, _ = db.Exec(`INSERT INTO posts (id, user_id, status, title, description, price, location) VALUES (5, 10, 'pending', 'T5', 'D', '100', 'L')`)

	os.Setenv("ADMINS", "99")
	LoadAdminsFromEnv()

	// approve
	resp := HandleAdminCommand(db, 99, "/pending 5 approve", "admin")
	if resp == "" {
		t.Fatalf("Expected response from approving pending post")
	}
	var status string
	_ = db.QueryRow("SELECT status FROM posts WHERE id = 5").Scan(&status)
	if status != StatusApproved {
		t.Fatalf("Expected status %s but got %s", StatusApproved, status)
	}

	// insert another pending and reject
	_, _ = db.Exec(`INSERT INTO posts (id, user_id, status, title) VALUES (6, 10, 'pending', 'T6')`)
	resp = HandleAdminCommand(db, 99, "/pending 6 reject", "admin")
	if resp == "" {
		t.Fatalf("Expected response from rejecting pending post")
	}
	_ = db.QueryRow("SELECT status FROM posts WHERE id = 6").Scan(&status)
	if status != StatusRejected {
		t.Fatalf("Expected status %s but got %s", StatusRejected, status)
	}
}
