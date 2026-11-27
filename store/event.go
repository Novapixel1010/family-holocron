package store

// Event types known to the core. Plugins may add more; unknown types are
// rejected by the API layer unless registered.
var EventTypes = []string{
	"marriage", "divorce", "engagement", "residence", "baptism", "christening", "burial",
	"cremation", "education", "graduation", "occupation", "military", "immigration",
	"emigration", "naturalization", "religion", "medical", "retirement", "census",
	"custom", "family", "historical",
}

// Event is a dated life or family milestone.
type Event struct {
	ID             int64              `json:"id"`
	Type           string             `json:"type"`
	Title          string             `json:"title"`
	Date           string             `json:"date"`
	SortDate       string             `json:"sortDate"`
	Place          string             `json:"place"`
	Description    string             `json:"description"`
	RelationshipID *int64             `json:"relationshipId"`
	Participants   []EventParticipant `json:"participants"`
	Attribution
}

// EventParticipant links a person to an event.
type EventParticipant struct {
	PersonID int64  `json:"personId"`
	Role     string `json:"role"` // principal, spouse, witness, …
	Name     string `json:"name"` // display name, filled on read
}

// EventInput holds editable event fields.
type EventInput struct {
	Type           string             `json:"type"`
	Title          string             `json:"title"`
	Date           string             `json:"date"`
	Place          string             `json:"place"`
	Description    string             `json:"description"`
	RelationshipID *int64             `json:"relationshipId"`
	Participants   []EventParticipant `json:"participants"`
}

// FindEvent filters events.
type FindEvent struct {
	IDs      []int64
	PersonID *int64
	Types    []string
	Query    string
	From     string // sort key lower bound (inclusive), e.g. "1850"
	To       string // sort key upper bound (inclusive)
}
