package fsm

type UserSession struct {
	UserID int64
	State  int
	// Draft holds a typed representation of the post being created.
	Draft *PostDraft
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

// PostDraft is a typed structure used while composing a post in the session.
type PostDraft struct {
	Title       string
	Description string
	Price       string
	Location    string
	Photos      []string
}
