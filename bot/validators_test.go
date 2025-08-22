package bot

import (
	"database/sql"
	"gosalebot/fsm"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestValidatePrice(t *testing.T) {
	var dbConn *sql.DB = nil // nil should use sensible defaults (validation enabled)
	ok, _ := ValidatePrice(dbConn, "123.45", "en")
	if !ok {
		t.Fatalf("expected numeric price to validate")
	}
	ok, _ = ValidatePrice(dbConn, "", "en")
	if ok {
		t.Fatalf("expected empty price to fail validation")
	}
	ok, _ = ValidatePrice(dbConn, "abc", "en")
	if ok {
		t.Fatalf("expected non-numeric price to fail validation")
	}
}

func TestValidatePhotos(t *testing.T) {
	var dbConn *sql.DB = nil
	ok, _ := ValidatePhotos(dbConn, []string{"p1"}, "en")
	if !ok {
		t.Fatalf("expected one photo to pass default validation")
	}
	ok, _ = ValidatePhotos(dbConn, []string{}, "en")
	if ok {
		t.Fatalf("expected zero photos to fail validation")
	}
}

// Integration: dispatcher should reject invalid price when user is in StatePrice
func TestDispatcherRejectsInvalidPrice(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	// create session and set state to price
	session := &fsm.UserSession{UserID: 999, State: fsm.StatePrice, Draft: &fsm.PostDraft{}}
	fsm.Sessions[999] = session

	update := models.Update{Message: &models.Message{From: &models.User{ID: 999}, Text: "notanumber"}}
	resp := HandleMessageWithDB(db, update, nil, 0, "en")
	if resp == "" {
		t.Fatalf("expected validation message, got empty")
	}
}
