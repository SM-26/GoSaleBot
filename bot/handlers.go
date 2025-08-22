package bot

import (
	"gosalebot/db"
	"gosalebot/fsm"
	"gosalebot/i18n"
	"log"
	"strings"

	"github.com/go-telegram/bot/models"
)

// Unified handlers file containing all per-state pure handlers.

// Pure handler: returns a HandlerResult describing what should happen.
func HandleTitleState(session *fsm.UserSession, msg *models.Message, lang string) HandlerResult {
	log.Printf("[INFO] User %d entered title: %s", session.UserID, msg.Text)
	next := fsm.StateDescription
	return HandlerResult{
		Text:      i18n.T(lang, "enter_description"),
		Set:       map[string]interface{}{"title": msg.Text},
		NextState: &next,
	}
}

func HandleDescriptionState(session *fsm.UserSession, msg *models.Message, lang string) HandlerResult {
	log.Printf("[INFO] User %d entered description: %s", session.UserID, msg.Text)
	next := fsm.StatePrice
	return HandlerResult{
		Text:      i18n.T(lang, "enter_price"),
		Set:       map[string]interface{}{"description": msg.Text},
		NextState: &next,
	}
}

func HandlePriceState(session *fsm.UserSession, msg *models.Message, lang string) HandlerResult {
	log.Printf("[INFO] User %d entered price: %s", session.UserID, msg.Text)
	next := fsm.StateLocation
	return HandlerResult{
		Text:      i18n.T(lang, "enter_location"),
		Set:       map[string]interface{}{"price": msg.Text},
		NextState: &next,
	}
}

func HandleLocationState(session *fsm.UserSession, msg *models.Message, lang string) HandlerResult {
	log.Printf("[INFO] User %d entered location: %s", session.UserID, msg.Text)
	next := fsm.StatePhotos
	return HandlerResult{
		Text:      i18n.T(lang, "send_photos"),
		Set:       map[string]interface{}{"location": msg.Text},
		NextState: &next,
	}
}

// HandlePhotosStatePure inspects the incoming update and returns a HandlerResult describing side-effects.
func HandlePhotosStatePure(session *fsm.UserSession, update models.Update, lang, saveUsername string) HandlerResult {
	msg := update.Message
	var photoFileIDs []string
	if msg != nil && len(msg.Photo) > 0 {
		photo := msg.Photo[len(msg.Photo)-1]
		photoFileIDs = append(photoFileIDs, photo.FileID)
	}

	if len(photoFileIDs) > 0 {
		log.Printf("[INFO] User %d sent %d photo(s)", session.UserID, len(photoFileIDs))
		return HandlerResult{
			Text:         i18n.T(lang, "photo_received"),
			AppendPhotos: photoFileIDs,
		}
	}

	text := ""
	if msg != nil {
		text = msg.Text
	}
	if strings.ToLower(text) == "done" {
		// Use typed Draft
		var title, description, price, location string
		var photos []string
		if session.Draft != nil {
			title = session.Draft.Title
			description = session.Draft.Description
			price = session.Draft.Price
			location = session.Draft.Location
			photos = session.Draft.Photos
		}

		post := db.Post{
			UserID:      session.UserID,
			ChatID:      0, // caller/executor will pass proper chat/message IDs if needed
			MessageID:   0,
			Title:       title,
			Description: description,
			Price:       price,
			Location:    location,
			Photos:      photos,
		}
		next := fsm.StateIdle
		return HandlerResult{
			Text:           i18n.T(lang, "post_submitted"),
			SavePost:       &post,
			SendModeration: true,
			NextState:      &next,
			ClearPostData:  true,
		}
	}

	return HandlerResult{Text: i18n.T(lang, "send_photo_or_done")}
}
