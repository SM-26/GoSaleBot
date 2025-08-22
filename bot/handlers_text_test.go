package bot

import (
	"gosalebot/fsm"
	"gosalebot/i18n"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestHandleTitleAndDescriptionHandlers(t *testing.T) {
	session := &fsm.UserSession{UserID: 42, State: fsm.StateTitle, Draft: &fsm.PostDraft{}}
	msg := &models.Message{Text: "A Great Item"}

	res := HandleTitleState(session, msg, "en")
	resp := ExecuteHandlerResult(nil, nil, session, res, 0, 0, "en", "")
	if session.State != fsm.StateDescription {
		t.Fatalf("expected state %d, got %d", fsm.StateDescription, session.State)
	}
	if session.Draft == nil || session.Draft.Title != "A Great Item" {
		t.Fatalf("expected title stored, got %v", session.Draft)
	}
	if resp != i18n.T("en", "enter_description") {
		t.Fatalf("unexpected response: %s", resp)
	}

	// Description
	session.State = fsm.StateDescription
	session.Draft = &fsm.PostDraft{}
	descMsg := &models.Message{Text: "This is a description."}
	res = HandleDescriptionState(session, descMsg, "en")
	resp = ExecuteHandlerResult(nil, nil, session, res, 0, 0, "en", "")
	if session.State != fsm.StatePrice {
		t.Fatalf("expected state %d, got %d", fsm.StatePrice, session.State)
	}
	if session.Draft == nil || session.Draft.Description != "This is a description." {
		t.Fatalf("expected description stored, got %v", session.Draft)
	}
	if resp != i18n.T("en", "enter_price") {
		t.Fatalf("unexpected response: %s", resp)
	}
}
