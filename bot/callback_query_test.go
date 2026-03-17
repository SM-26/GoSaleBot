package bot

import (
	"gosalebot/fsm"
	"log"
	"os"
	"testing"

	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func TestHandleCallbackQuery_ApproveReject(t *testing.T) {
	db := newTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()
	// Mock bot
	mockBot, _ := telegram.New("dummy_token", telegram.WithHTTPClient(0, &mockTelegramHttpClient{}))
	SetGlobalBotInstance(mockBot) // Needed for Approve/Reject notifications

	// Insert a pending post
	_, err := db.Exec(`INSERT INTO posts (id, user_id, status, title) VALUES (1, 123, 'pending', 'Test Post')`)
	if err != nil {
		t.Fatalf("Failed to insert test post: %v", err)
	}
	_, err = db.Exec(`INSERT INTO users (id, username) VALUES (123, 'testuser')`)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	// Simulate an "approve" callback
	approveUpdate := models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "1",
			From: models.User{ID: 456}, // Admin
			Data: "approve:1",
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{
					ID:   100,
					Chat: models.Chat{ID: -1000},
				},
			},
		},
	}

	HandleCallbackQuery(db, approveUpdate, mockBot, -2000, 0)

	// Verify post is approved
	var status string
	err = db.QueryRow("SELECT status FROM posts WHERE id = 1").Scan(&status)
	if err != nil {
		t.Fatalf("Failed to query post status: %v", err)
	}
	if status != StatusApproved {
		t.Errorf("Expected status to be 'approved', got '%s'", status)
	}

	// Insert another pending post
	_, err = db.Exec(`INSERT INTO posts (id, user_id, status, title) VALUES (2, 123, 'pending', 'Test Post 2')`)
	if err != nil {
		t.Fatalf("Failed to insert test post: %v", err)
	}

	// Simulate a "reject" callback
	rejectUpdate := models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "2",
			From: models.User{ID: 456}, // Admin
			Data: "reject:2",
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{
					ID:   101,
					Chat: models.Chat{ID: -1000},
				},
			},
		},
	}

	HandleCallbackQuery(db, rejectUpdate, mockBot, -2000, 0)

	// Verify post is rejected
	err = db.QueryRow("SELECT status FROM posts WHERE id = 2").Scan(&status)
	if err != nil {
		t.Fatalf("Failed to query post status: %v", err)
	}
	if status != StatusRejected {
		t.Errorf("Expected status to be 'rejected', got '%s'", status)
	}
}

func TestHandleCallbackQuery_Done(t *testing.T) {
	db := newTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()
	// Mock bot
	mockBot, _ := telegram.New("dummy_token", telegram.WithHTTPClient(0, &mockTelegramHttpClient{}))
	SetGlobalBotInstance(mockBot)

	// Setup session
	session := &fsm.UserSession{
		UserID: 123,
		State:  fsm.StatePhotos,
		Draft: &fsm.PostDraft{
			Title:  "Final Post",
			Photos: []string{"file1"},
		},
	}
	fsm.Sessions[123] = session

	// Simulate a "done" callback
	doneUpdate := models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "3",
			From: models.User{ID: 123},
			Data: "done",
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{
					ID:   200,
					Chat: models.Chat{ID: 123},
				},
			},
		},
	}

	// Set required env vars for the handler
	_ = os.Setenv("MODERATION_GROUP_ID", "-1000")
	_ = os.Setenv("MODERATION_TOPIC_ID", "0")

	HandleCallbackQuery(db, doneUpdate, mockBot, -2000, 0)

	// Verify post was created and session was reset
	var postID int
	err := db.QueryRow("SELECT id FROM posts WHERE title = 'Final Post'").Scan(&postID)
	if err != nil {
		t.Fatalf("Expected post to be created, but got error: %v", err)
	}

	if fsm.Sessions[123].State != fsm.StateIdle {
		t.Errorf("Expected user state to be reset to Idle, but was %d", fsm.Sessions[123].State)
	}
	if fsm.Sessions[123].Draft != nil {
		t.Errorf("Expected user draft to be cleared, but it was not")
	}
}
