package store

// Sex values follow GEDCOM (X = other / intersex, U = unknown).
const (
	SexMale    = "M"
	SexFemale  = "F"
	SexOther   = "X"
	SexUnknown = "U"
)

// Person is an individual in the family tree.
type Person struct {
	ID              int64  `json:"id"`
	UID             string `json:"uid"`
	GivenName       string `json:"givenName"`
	Surname         string `json:"surname"`
	Nickname        string `json:"nickname"`
	Sex             string `json:"sex"`
	BirthDate       string `json:"birthDate"`
	BirthPlace      string `json:"birthPlace"`
	DeathDate       string `json:"deathDate"`
	DeathPlace      string `json:"deathPlace"`
	Deceased        bool   `json:"deceased"`
	About           string `json:"about"` // Markdown biography / family stories
	PortraitMediaID *int64 `json:"portraitMediaId"`
	// Derived fields (not stored as-is).
	BirthSort string `json:"birthSort"`
	DeathSort string `json:"deathSort"`
	Attribution
}

// DisplayName returns "Given Surname" with a fallback.
func (p *Person) DisplayName() string {
	n := p.GivenName
	if p.Surname != "" {
		if n != "" {
			n += " "
		}
		n += p.Surname
	}
	if n == "" {
		return "Unnamed person"
	}
	return n
}

// IsLiving reports whether the person is presumed living.
func (p *Person) IsLiving() bool { return !p.Deceased && p.DeathDate == "" }

// PersonInput holds the editable fields of a person.
type PersonInput struct {
	UID             string `json:"uid,omitempty"` // optional on create (generated)
	GivenName       string `json:"givenName"`
	Surname         string `json:"surname"`
	Nickname        string `json:"nickname"`
	Sex             string `json:"sex"`
	BirthDate       string `json:"birthDate"`
	BirthPlace      string `json:"birthPlace"`
	DeathDate       string `json:"deathDate"`
	DeathPlace      string `json:"deathPlace"`
	Deceased        bool   `json:"deceased"`
	About           string `json:"about"`
	PortraitMediaID *int64 `json:"portraitMediaId"`
}

// PersonSort selects list ordering.
type PersonSort string

const (
	SortByName    PersonSort = "name"
	SortByBirth   PersonSort = "birth"
	SortByUpdated PersonSort = "updated"
)

// FindPerson filters persons.
type FindPerson struct {
	IDs    []int64
	Query  string // matches given name, surname, nickname, places
	Sort   PersonSort
	Limit  int
	Offset int
}
