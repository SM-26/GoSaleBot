package bot

import (
	"database/sql"
	"gosalebot/db"
	"gosalebot/fsm"

	telegram "github.com/go-telegram/bot"
)

// HandlerResult is a small action/result object returned by pure handlers.
type HandlerResult struct {
	Text           string
	Set            map[string]interface{} // key/values to set in session.Draft where applicable (title, description, price, location, photos)
	AppendPhotos   []string               // photos to append
	NextState      *int                   // if non-nil, set session.State
	SavePost       *db.Post               // if non-nil, executor will persist this post
	SendModeration bool                   // if true and SavePost != nil, executor will send moderation message
	ClearPostData  bool                   // if true, session.Draft will be reset
}

// ExecuteHandlerResult applies the HandlerResult side-effects: mutate session, save to DB, send moderation messages.
// It is resilient to nil dbConn or nil bot for tests.
func ExecuteHandlerResult(dbConn *sql.DB, bot *telegram.Bot, session *fsm.UserSession, res HandlerResult, moderationGroupID int64, moderationTopicID int, lang, saveUsername string) string {
	// Apply Set into typed Draft
	if res.Set != nil {
		if session.Draft == nil {
			session.Draft = &fsm.PostDraft{}
		}
		for k, v := range res.Set {
			switch k {
			case "title":
				if s, ok := v.(string); ok {
					session.Draft.Title = s
				}
			case "description":
				if s, ok := v.(string); ok {
					session.Draft.Description = s
				}
			case "price":
				if s, ok := v.(string); ok {
					session.Draft.Price = s
				}
			case "location":
				if s, ok := v.(string); ok {
					session.Draft.Location = s
				}
			case "photos":
				if ps, ok := v.([]string); ok {
					session.Draft.Photos = ps
				}
			default:
				// unknown keys are ignored under typed migration
			}
		}
	}
	// Append photos into Draft
	if len(res.AppendPhotos) > 0 {
		if session.Draft == nil {
			session.Draft = &fsm.PostDraft{}
		}
		session.Draft.Photos = append(session.Draft.Photos, res.AppendPhotos...)
	}
	// Save post if requested
	if res.SavePost != nil && dbConn != nil {
		postID, err := db.SavePostToDB(dbConn, *res.SavePost)
		if err == nil {
			if res.SendModeration && bot != nil {
				// rely on existing sendModerationMessage helper
				sendModerationMessage(dbConn, bot, session, postID, moderationGroupID, moderationTopicID, lang, saveUsername)
			}
		}
	}
	// Update state
	if res.NextState != nil {
		session.State = *res.NextState
	}
	if res.ClearPostData {
		session.Draft = nil
	}
	return res.Text
}
