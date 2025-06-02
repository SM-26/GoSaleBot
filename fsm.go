package fsm

type UserSession struct {
	UserID   int64
	State    int
	PostData map[string]interface{}
}

const (
	StateIdle = iota
	StateTitle
	StateDescription
	StatePrice
	StateLocation
	StatePhotos
	StatePreview
)

var Sessions = make(map[int64]*UserSession)
