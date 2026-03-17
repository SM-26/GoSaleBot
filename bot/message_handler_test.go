package bot

import (
	// "context"
	"database/sql"
	"gosalebot/fsm"
	"gosalebot/i18n"
	"log"
	"net/http" // Added for mocking http.Client
	"os"
	"testing"

	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	// "github.com/mattn/go-sqlite3"
)

type mockTelegramHttpClient struct{}

func (m *mockTelegramHttpClient) Do(req *http.Request) (*http.Response, error) {
	// Return a dummy response
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}, nil
}

// type mockRoundTripper struct{}

// func (m *mockRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
// 	// Return a dummy response
// 	return &http.Response{
// 		StatusCode: http.StatusOK,
// 		Body:       http.NoBody,
// 	}, nil
// }

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// Use in-memory SQLite database for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Create schema
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

func TestHandleIdleState(t *testing.T) {
	db := newTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	session := &fsm.UserSession{UserID: 123, State: fsm.StateIdle}

	// Test with /start command
	msg := &models.Message{Text: "/start", Chat: models.Chat{ID: 123}}
	response := handleIdleState(db, session, msg, "en", "testuser")

	if session.State != fsm.StateTitle {
		t.Errorf("Expected state to be %d, but got %d", fsm.StateTitle, session.State)
	}

	expectedResponse := i18n.T("en", "welcome")
	if response != expectedResponse {
		t.Errorf("Expected response to be '%s', but got '%s'", expectedResponse, response)
	}

	// Test with a random message in a private chat
	session.State = fsm.StateIdle
	msg = &models.Message{Text: "hello", Chat: models.Chat{ID: 123}}
	response = handleIdleState(db, session, msg, "en", "testuser")

	expectedResponse = i18n.T("en", "start")
	if response != expectedResponse {
		t.Errorf("Expected response to be '%s', but got '%s'", expectedResponse, response)
	}
}

func TestHandleTitleState(t *testing.T) {

	session := &fsm.UserSession{UserID: 123, State: fsm.StateTitle, Draft: &fsm.PostDraft{}}
	msg := &models.Message{Text: "Test Title"}

	res := HandleTitleState(session, msg, "en")
	response := ExecuteHandlerResult(nil, nil, session, res, 0, 0, "en", "")

	if session.State != fsm.StateDescription {
		t.Errorf("Expected state to be %d, but got %d", fsm.StateDescription, session.State)
	}

	if session.Draft == nil || session.Draft.Title != "Test Title" {
		t.Errorf("Expected title to be 'Test Title', but got '%v'", session.Draft)
	}

	expectedResponse := i18n.T("en", "enter_description")
	if response != expectedResponse {
		t.Errorf("Expected response to be '%s', but got '%s'", expectedResponse, response)
	}
}

func TestHandleDescriptionState(t *testing.T) {

	session := &fsm.UserSession{UserID: 123, State: fsm.StateDescription, Draft: &fsm.PostDraft{}}
	msg := &models.Message{Text: "Test Description"}

	res := HandleDescriptionState(session, msg, "en")
	response := ExecuteHandlerResult(nil, nil, session, res, 0, 0, "en", "")

	if session.State != fsm.StatePrice {
		t.Errorf("Expected state to be %d, but got %d", fsm.StatePrice, session.State)
	}

	if session.Draft == nil || session.Draft.Description != "Test Description" {
		t.Errorf("Expected description to be 'Test Description', but got '%v'", session.Draft)
	}

	expectedResponse := i18n.T("en", "enter_price")
	if response != expectedResponse {
		t.Errorf("Expected response to be '%s', but got '%s'", expectedResponse, response)
	}
}

func TestHandlePriceState(t *testing.T) {

	session := &fsm.UserSession{UserID: 123, State: fsm.StatePrice, Draft: &fsm.PostDraft{}}
	msg := &models.Message{Text: "100"}

	res := HandlePriceState(session, msg, "en")
	response := ExecuteHandlerResult(nil, nil, session, res, 0, 0, "en", "")

	if session.State != fsm.StateLocation {
		t.Errorf("Expected state to be %d, but got %d", fsm.StateLocation, session.State)
	}

	if session.Draft == nil || session.Draft.Price != "100" {
		t.Errorf("Expected price to be '100', but got '%v'", session.Draft)
	}

	expectedResponse := i18n.T("en", "enter_location")
	if response != expectedResponse {
		t.Errorf("Expected response to be '%s', but got '%s'", expectedResponse, response)
	}
}

func TestHandleLocationState(t *testing.T) {

	session := &fsm.UserSession{UserID: 123, State: fsm.StateLocation, Draft: &fsm.PostDraft{}}
	msg := &models.Message{Text: "Test Location"}

	res := HandleLocationState(session, msg, "en")
	response := ExecuteHandlerResult(nil, nil, session, res, 0, 0, "en", "")

	if session.State != fsm.StatePhotos {
		t.Errorf("Expected state to be %d, but got %d", fsm.StatePhotos, session.State)
	}

	if session.Draft == nil || session.Draft.Location != "Test Location" {
		t.Errorf("Expected location to be 'Test Location', but got '%v'", session.Draft)
	}

	expectedResponse := i18n.T("en", "send_photos")
	if response != expectedResponse {
		t.Errorf("Expected response to be '%s', but got '%s'", expectedResponse, response)
	}
}

func TestHandlePhotosState(t *testing.T) {
	db := newTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()
	// Mock bot for SendMessage and SendPhoto
	// Initialize with a dummy http.Client to prevent nil pointer dereference
	mockBot, _ := telegram.New("dummy_token", telegram.WithHTTPClient(0, &mockTelegramHttpClient{}))

	// Setup initial session data for a post
	session := &fsm.UserSession{
		UserID: 123,
		State:  fsm.StatePhotos,
		Draft: &fsm.PostDraft{
			Title:       "Test Title",
			Description: "Test Description",
			Price:       "100",
			Location:    "Test Location",
		},
	}

	moderationGroupID := int64(-100123456789)
	moderationTopicID := 123
	saveUsername := "testuser"

	// Test sending a photo
	updateWithPhoto := models.Update{
		Message: &models.Message{
			From:  &models.User{ID: 123},
			Photo: []models.PhotoSize{{FileID: "photo1"}},
			Chat:  models.Chat{ID: 123},
		},
	}
	res := HandlePhotosStatePure(session, updateWithPhoto, "en", saveUsername)
	response := ExecuteHandlerResult(db, mockBot, session, res, moderationGroupID, moderationTopicID, "en", saveUsername)

	if response != i18n.T("en", "photo_received") {
		t.Errorf("Expected 'photo_received', got '%s'", response)
	}
	if session.Draft == nil || len(session.Draft.Photos) != 1 || session.Draft.Photos[0] != "photo1" {
		t.Errorf("Expected photo1 to be stored, got %v", session.Draft)
	}

	// Test sending another photo
	updateWithPhoto2 := models.Update{
		Message: &models.Message{
			From:  &models.User{ID: 123},
			Photo: []models.PhotoSize{{FileID: "photo2"}},
			Chat:  models.Chat{ID: 123},
		},
	}
	res = HandlePhotosStatePure(session, updateWithPhoto2, "en", saveUsername)
	response = ExecuteHandlerResult(db, mockBot, session, res, moderationGroupID, moderationTopicID, "en", saveUsername)

	if response != i18n.T("en", "photo_received") {
		t.Errorf("Expected 'photo_received', got '%s'", response)
	}
	if session.Draft == nil || len(session.Draft.Photos) != 2 || session.Draft.Photos[0] != "photo1" || session.Draft.Photos[1] != "photo2" {
		t.Errorf("Expected photo1, photo2 to be stored, got %v", session.Draft)
	}

	// Test sending "done" command
	updateDone := models.Update{
		Message: &models.Message{
			From: &models.User{ID: 123},
			Text: "done",
			Chat: models.Chat{ID: 123, Type: "private"},
			ID:   456, // Message ID for moderation_message_id
		},
	}
	res = HandlePhotosStatePure(session, updateDone, "en", saveUsername)
	response = ExecuteHandlerResult(db, mockBot, session, res, moderationGroupID, moderationTopicID, "en", saveUsername)

	if session.State != fsm.StateIdle {
		t.Errorf("Expected state to be %d, but got %d", fsm.StateIdle, session.State)
	}
	if session.Draft != nil {
		t.Errorf("Expected Draft to be nil after clearing, got %v", session.Draft)
	}
	if response != i18n.T("en", "post_submitted") {
		t.Errorf("Expected 'post_submitted', got '%s'", response)
	}

	// Verify post saved in DB
	var status string
	var moderationMessageID int64
	var moderationPhotoMessageIDs string
	err := db.QueryRow("SELECT status, moderation_message_id, moderation_photo_message_ids FROM posts WHERE user_id = ?", 123).Scan(&status, &moderationMessageID, &moderationPhotoMessageIDs)
	if err != nil {
		t.Fatalf("Failed to query post: %v", err)
	}
	if status != StatusPending {
		t.Errorf("Expected post status to be %s, got %s", StatusPending, status)
	}
	// moderationMessageID and moderationPhotoMessageIDs will be 0 and empty string respectively
	// because we are using a mock bot that doesn't actually send messages and return message IDs.
	// This is acceptable for unit testing the state logic.

	// Verify photos saved in DB
	var photoCount int
	err = db.QueryRow("SELECT COUNT(*) FROM photos WHERE post_id = (SELECT id FROM posts WHERE user_id = ?)", 123).Scan(&photoCount)
	if err != nil {
		t.Fatalf("Failed to query photo count: %v", err)
	}
	if photoCount != 2 {
		t.Errorf("Expected 2 photos, got %d", photoCount)
	}

	// Test invalid input in photo state
	session.State = fsm.StatePhotos
	updateInvalid := models.Update{
		Message: &models.Message{
			From: &models.User{ID: 123},
			Text: "invalid input",
			Chat: models.Chat{ID: 123},
		},
	}
	res = HandlePhotosStatePure(session, updateInvalid, "en", saveUsername)
	response = ExecuteHandlerResult(db, mockBot, session, res, moderationGroupID, moderationTopicID, "en", saveUsername)
	if response != i18n.T("en", "send_photo_or_done") {
		t.Errorf("Expected 'send_photo_or_done', got '%s'", response)
	}
}

func TestHandleRejectViaReply(t *testing.T) {
	db := newTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()
	// Insert a pending post for testing rejection
	postID := int64(1)
	userID := int64(123)
	moderationMsgID := int64(98765)
	_, err := db.Exec(`INSERT INTO posts (id, user_id, chat_id, message_id, status, title, description, price, location, created_at, expires_at, moderation_message_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now', '+1 day'), ?)`,
		postID, userID, 0, 0, StatusPending, "Test Title", "Test Desc", "100", "Test Loc", moderationMsgID)
	if err != nil {
		t.Fatalf("Failed to insert test post: %v", err)
	}

	// Set up admin user
	_ = os.Setenv("ADMINS", "456")
	LoadAdminsFromEnv()

	moderationGroupID := int64(-100123456789)

	// Simulate admin reply to moderation message
	adminMsg := &models.Message{
		From: &models.User{ID: 456}, // Admin user
		Text: "Reason: Spam",
		Chat: models.Chat{ID: moderationGroupID}, // Reply in moderation group
		ReplyToMessage: &models.Message{
			ID:   int(moderationMsgID),
			Chat: models.Chat{ID: moderationGroupID},
		},
	}

	response := handleRejectViaReply(db, adminMsg, moderationGroupID)

	if response != "Post rejected with custom reason." {
		t.Errorf("Expected 'Post rejected with custom reason.', got '%s'", response)
	}

	// Verify post status updated to rejected
	var status string
	err = db.QueryRow("SELECT status FROM posts WHERE id = ?", postID).Scan(&status)
	if err != nil {
		t.Fatalf("Failed to query post status: %v", err)
	}
	if status != StatusRejected {
		t.Errorf("Expected post status to be %s, got %s", StatusRejected, status)
	}

	// Verify photos are deleted (if any)
	var photoCount int
	err = db.QueryRow("SELECT COUNT(*) FROM photos WHERE post_id = ?", postID).Scan(&photoCount)
	if err != nil {
		t.Fatalf("Failed to query photo count: %v", err)
	}
	if photoCount != 0 {
		t.Errorf("Expected 0 photos, got %d", photoCount)
	}
}
