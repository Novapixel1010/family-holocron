package store

// Note is a free-form Markdown note attached to a person.
type Note struct {
	ID       int64  `json:"id"`
	PersonID int64  `json:"personId"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	Attribution
}

// NoteInput holds editable note fields.
type NoteInput struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}
