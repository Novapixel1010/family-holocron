package store

// Media is an uploaded file (photo, document, recording).
type Media struct {
	ID           int64   `json:"id"`
	UID          string  `json:"uid"`
	StoragePath  string  `json:"-"` // relative to the media directory
	OriginalName string  `json:"originalName"`
	MimeType     string  `json:"mimeType"`
	SizeBytes    int64   `json:"sizeBytes"`
	SHA256       string  `json:"sha256"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	TakenDate    string  `json:"takenDate"`
	PersonIDs    []int64 `json:"personIds"`
	Attribution
}

// MediaInput holds editable media metadata.
type MediaInput struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	TakenDate   string  `json:"takenDate"`
	PersonIDs   []int64 `json:"personIds"`
}

// FindMedia filters media.
type FindMedia struct {
	PersonID *int64
	Query    string
	Limit    int
	Offset   int
}
