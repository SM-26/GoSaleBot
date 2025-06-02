package main

import (
	"database/sql"
	"gosalebot/bot"
	"gosalebot/db"
	"gosalebot/fsm"
	"os"
	"strconv"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestGroupIDEnvParsing(t *testing.T) {
	os.Setenv("MODERATION_GROUP_ID", "-1001234567890")
	os.Setenv("APPROVED_GROUP_ID", "-1009876543210")

	modGroup := os.Getenv("MODERATION_GROUP_ID")
	approvedGroup := os.Getenv("APPROVED_GROUP_ID")

	modID, err := strconv.ParseInt(modGroup, 10, 64)
	if err != nil {
		t.Fatalf("Failed to parse MODERATION_GROUP_ID: %v", err)
	}
	if modID != -1001234567890 {
		t.Errorf("Expected MODERATION_GROUP_ID to be -1001234567890, got %d", modID)
	}

	appID, err := strconv.ParseInt(approvedGroup, 10, 64)
	if err != nil {
		t.Fatalf("Failed to parse APPROVED_GROUP_ID: %v", err)
	}
	if appID != -1009876543210 {
		t.Errorf("Expected APPROVED_GROUP_ID to be -1009876543210, got %d", appID)
	}
}

func TestSavePostToDB(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory DB: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		status TEXT NOT NULL CHECK(status IN ('pending', 'approved', 'rejected')),
		title TEXT,
		description TEXT,
		price TEXT,
		location TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME
	)`)
	if err != nil {
		t.Fatalf("Failed to create posts table: %v", err)
	}

	postData := map[string]interface{}{
		"title":       "Test Title",
		"description": "Test Description",
		"price":       "$10",
		"location":    "Test City",
	}
	err = savePostToDB(db, 42, postData)
	if err != nil {
		t.Fatalf("savePostToDB failed: %v", err)
	}

	row := db.QueryRow("SELECT user_id, status, title, description, price, location FROM posts WHERE user_id = ?", 42)
	var userID int64
	var status, title, description, price, location string
	err = row.Scan(&userID, &status, &title, &description, &price, &location)
	if err != nil {
		t.Fatalf("Failed to query saved post: %v", err)
	}
	if userID != 42 || status != "pending" || title != "Test Title" || description != "Test Description" || price != "$10" || location != "Test City" {
		t.Errorf("Saved post fields do not match expected values")
	}
}

func setupTestDB(t *testing.T) *sql.DB {
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory DB: %v", err)
	}
	_, err = dbConn.Exec(`CREATE TABLE posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		status TEXT NOT NULL CHECK(status IN ('pending', 'approved', 'rejected')),
		title TEXT,
		description TEXT,
		price TEXT,
		location TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME
	)`)
	if err != nil {
		t.Fatalf("Failed to create posts table: %v", err)
	}
	_, err = dbConn.Exec(`CREATE TABLE photos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
		file_id TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("Failed to create photos table: %v", err)
	}
	_, err = dbConn.Exec(`CREATE TABLE config (
		key TEXT PRIMARY KEY,
		value TEXT
	)`)
	if err != nil {
		t.Fatalf("Failed to create config table: %v", err)
	}
	return dbConn
}

func TestConfigHelpers(t *testing.T) {
	dbConn := setupTestDB(t)
	defer dbConn.Close()
	err := db.SetConfig(dbConn, "FOO", "BAR")
	if err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}
	val, err := db.GetConfig(dbConn, "FOO")
	if err != nil || val != "BAR" {
		t.Errorf("GetConfig failed, got %v want BAR", val)
	}
}

func TestAdminCommandConfig(t *testing.T) {
	dbConn := setupTestDB(t)
	defer dbConn.Close()
	resp := bot.HandleAdminCommand(dbConn, 123456789, "/config FOO BAR")
	if resp != "Config updated: FOO = BAR" {
		t.Errorf("unexpected response: %s", resp)
	}
	resp = bot.HandleAdminCommand(dbConn, 123456789, "/config")
	if resp == "" || resp == "Unknown admin command." {
		t.Errorf("expected config output, got: %s", resp)
	}
}

func TestAdminCommandPending(t *testing.T) {
	dbConn := setupTestDB(t)
	defer dbConn.Close()
	// Insert a pending post
	_, err := dbConn.Exec(`INSERT INTO posts (user_id, status, title, description, price, location, created_at, expires_at) VALUES (1, 'pending', 'Test', 'Desc', '10', 'Loc', datetime('now'), datetime('now', '+24 hours'))`)
	if err != nil {
		t.Fatalf("Failed to insert post: %v", err)
	}
	resp := bot.HandleAdminCommand(dbConn, 123456789, "/pending")
	if resp == "" || resp == "Unknown admin command." {
		t.Errorf("expected pending output, got: %s", resp)
	}
}

func TestEnvParsingAndTimeout(t *testing.T) {
	os.Setenv("MODERATION_GROUP_ID", "-1001234567890")
	os.Setenv("APPROVED_GROUP_ID", "-1009876543210")
	os.Setenv("TIMEOUT_MINUTES", "60")
	modGroup := os.Getenv("MODERATION_GROUP_ID")
	approvedGroup := os.Getenv("APPROVED_GROUP_ID")
	timeoutStr := os.Getenv("TIMEOUT_MINUTES")
	modID, err := strconv.ParseInt(modGroup, 10, 64)
	if err != nil || modID != -1001234567890 {
		t.Errorf("MODERATION_GROUP_ID parse failed")
	}
	appID, err := strconv.ParseInt(approvedGroup, 10, 64)
	if err != nil || appID != -1009876543210 {
		t.Errorf("APPROVED_GROUP_ID parse failed")
	}
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil || timeout != 60 {
		t.Errorf("TIMEOUT_MINUTES parse failed")
	}
}

func TestFSMUserFlow(t *testing.T) {
	// This test simulates a full user flow through the FSM: title → description → price → location → photos → preview → confirm/cancel
	// It uses the real FSM and bot logic, with an in-memory DB and a fake bot (nil for tgbotapi.BotAPI, since we don't send real messages)
	// We check state transitions and output messages.

	dbConn := setupTestDB(t)
	defer dbConn.Close()
	userID := int64(555)
	moderationGroupID := int64(-1001)
	lang := "en"

	// Reset FSM session
	if session, ok := fsm.Sessions[userID]; ok {
		session.State = fsm.StateIdle
		session.PostData = make(map[string]interface{})
	} else {
		fsm.Sessions[userID] = &fsm.UserSession{UserID: userID, State: fsm.StateIdle, PostData: make(map[string]interface{})}
	}

	// 1. Start
	resp := bot.HandleMessageWithDB(dbConn, userID, "/start", nil, 0, 0, nil, moderationGroupID, lang)
	if resp == "" || fsm.Sessions[userID].State != fsm.StateTitle {
		t.Fatalf("Expected welcome message and StateTitle, got: %q, state=%d", resp, fsm.Sessions[userID].State)
	}

	// 2. Title
	resp = bot.HandleMessageWithDB(dbConn, userID, "My Item", nil, 0, 0, nil, moderationGroupID, lang)
	if resp == "" || fsm.Sessions[userID].State != fsm.StateDescription {
		t.Fatalf("Expected description prompt and StateDescription, got: %q, state=%d", resp, fsm.Sessions[userID].State)
	}

	// 3. Description
	resp = bot.HandleMessageWithDB(dbConn, userID, "A great item", nil, 0, 0, nil, moderationGroupID, lang)
	if resp == "" || fsm.Sessions[userID].State != fsm.StatePrice {
		t.Fatalf("Expected price prompt and StatePrice, got: %q, state=%d", resp, fsm.Sessions[userID].State)
	}

	// 4. Price
	resp = bot.HandleMessageWithDB(dbConn, userID, "$42", nil, 0, 0, nil, moderationGroupID, lang)
	if resp == "" || fsm.Sessions[userID].State != fsm.StateLocation {
		t.Fatalf("Expected location prompt and StateLocation, got: %q, state=%d", resp, fsm.Sessions[userID].State)
	}

	// 5. Location
	resp = bot.HandleMessageWithDB(dbConn, userID, "Tel Aviv", nil, 0, 0, nil, moderationGroupID, lang)
	if resp == "" || fsm.Sessions[userID].State != fsm.StatePhotos {
		t.Fatalf("Expected photo prompt and StatePhotos, got: %q, state=%d", resp, fsm.Sessions[userID].State)
	}

	// 6. Add photo
	photoIDs := []string{"photo_file_id_1"}
	resp = bot.HandleMessageWithDB(dbConn, userID, "", nil, 0, 0, photoIDs, moderationGroupID, lang)
	if resp == "" || fsm.Sessions[userID].State != fsm.StatePhotos {
		t.Fatalf("Expected photo received message and StatePhotos, got: %q, state=%d", resp, fsm.Sessions[userID].State)
	}
	if photos, ok := fsm.Sessions[userID].PostData["photos"].([]string); !ok || len(photos) != 1 {
		t.Fatalf("Expected 1 photo in session, got: %v", fsm.Sessions[userID].PostData["photos"])
	}

	// 7. Done with photos
	resp = bot.HandleMessageWithDB(dbConn, userID, "done", nil, 0, 0, nil, moderationGroupID, lang)
	if resp == "" || fsm.Sessions[userID].State != fsm.StatePreview {
		t.Fatalf("Expected preview message and StatePreview, got: %q, state=%d", resp, fsm.Sessions[userID].State)
	}
	if want := "Preview:"; resp[:len(want)] != want {
		t.Errorf("Expected preview message, got: %q", resp)
	}

	// 8. Confirm
	resp = bot.HandleMessageWithDB(dbConn, userID, "confirm", nil, 0, 0, nil, moderationGroupID, lang)
	if resp == "" || fsm.Sessions[userID].State != fsm.StateIdle {
		t.Fatalf("Expected post submitted message and StateIdle, got: %q, state=%d", resp, fsm.Sessions[userID].State)
	}
	if want := "Post submitted for moderation!"; resp != want {
		t.Errorf("Expected submission confirmation, got: %q", resp)
	}

	// 9. Cancel flow (should reset to idle)
	fsm.Sessions[userID].State = fsm.StatePreview
	resp = bot.HandleMessageWithDB(dbConn, userID, "cancel", nil, 0, 0, nil, moderationGroupID, lang)
	if resp == "" || fsm.Sessions[userID].State != fsm.StateIdle {
		t.Fatalf("Expected post cancelled message and StateIdle, got: %q, state=%d", resp, fsm.Sessions[userID].State)
	}
}
