package store

import (
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	maxShort = 200
	maxLong  = 200_000 // Markdown bodies
)

var (
	extraEventTypesMu sync.RWMutex
	extraEventTypes   = map[string]bool{}
)

// RegisterEventType lets plugins introduce additional event types.
func RegisterEventType(t string) {
	extraEventTypesMu.Lock()
	defer extraEventTypesMu.Unlock()
	extraEventTypes[t] = true
}

// AllEventTypes returns core and plugin-registered event types.
func AllEventTypes() []string {
	out := slices.Clone(EventTypes)
	extraEventTypesMu.RLock()
	defer extraEventTypesMu.RUnlock()
	for t := range extraEventTypes {
		out = append(out, t)
	}
	return out
}

// IsEventType reports whether t is a known event type.
func IsEventType(t string) bool {
	if slices.Contains(EventTypes, t) {
		return true
	}
	extraEventTypesMu.RLock()
	defer extraEventTypesMu.RUnlock()
	return extraEventTypes[t]
}

type validator struct{ fields map[string]string }

func (v *validator) check(cond bool, field, msg string) {
	if !cond {
		if v.fields == nil {
			v.fields = map[string]string{}
		}
		if _, ok := v.fields[field]; !ok {
			v.fields[field] = msg
		}
	}
}

func (v *validator) short(field, s string) {
	v.check(utf8.RuneCountInString(s) <= maxShort, field, "must be at most 200 characters")
}

func (v *validator) long(field, s string) {
	v.check(len(s) <= maxLong, field, "is too long")
}

func (v *validator) err() error {
	if len(v.fields) == 0 {
		return nil
	}
	return &ValidationError{Message: "Please correct the highlighted fields.", Fields: v.fields}
}

// Normalize trims and validates a person input.
func (in *PersonInput) Normalize() error {
	in.GivenName = strings.TrimSpace(in.GivenName)
	in.Surname = strings.TrimSpace(in.Surname)
	in.Nickname = strings.TrimSpace(in.Nickname)
	in.BirthDate = strings.TrimSpace(in.BirthDate)
	in.BirthPlace = strings.TrimSpace(in.BirthPlace)
	in.DeathDate = strings.TrimSpace(in.DeathDate)
	in.DeathPlace = strings.TrimSpace(in.DeathPlace)
	in.UID = strings.TrimSpace(in.UID)
	if in.Sex == "" {
		in.Sex = SexUnknown
	}
	in.Sex = strings.ToUpper(in.Sex)
	if in.DeathDate != "" || in.DeathPlace != "" {
		in.Deceased = true
	}
	v := &validator{}
	v.check(in.GivenName != "" || in.Surname != "", "givenName", "a given name or surname is required")
	for f, s := range map[string]string{"givenName": in.GivenName, "surname": in.Surname, "nickname": in.Nickname,
		"birthDate": in.BirthDate, "birthPlace": in.BirthPlace, "deathDate": in.DeathDate, "deathPlace": in.DeathPlace, "uid": in.UID} {
		v.short(f, s)
	}
	v.check(slices.Contains([]string{SexMale, SexFemale, SexOther, SexUnknown}, in.Sex), "sex", "must be M, F, X or U")
	v.long("about", in.About)
	return v.err()
}

// Normalize trims and validates a relationship input.
func (in *RelationshipInput) Normalize() error {
	in.Type = strings.TrimSpace(in.Type)
	in.Subtype = strings.TrimSpace(in.Subtype)
	in.StartDate = strings.TrimSpace(in.StartDate)
	in.EndDate = strings.TrimSpace(in.EndDate)
	v := &validator{}
	v.check(in.Type == RelParent || in.Type == RelSpouse, "type", "must be parent or spouse")
	v.check(in.Person1ID > 0, "person1Id", "is required")
	v.check(in.Person2ID > 0, "person2Id", "is required")
	v.check(in.Person1ID != in.Person2ID, "person2Id", "a person cannot be related to themselves")
	switch in.Type {
	case RelParent:
		if in.Subtype == "" {
			in.Subtype = "biological"
		}
		v.check(slices.Contains(ParentSubtypes, in.Subtype), "subtype", "unknown parent relationship type")
	case RelSpouse:
		if in.Subtype == "" {
			in.Subtype = "married"
		}
		v.check(slices.Contains(SpouseSubtypes, in.Subtype), "subtype", "unknown partner relationship type")
		if in.Person1ID > in.Person2ID {
			in.Person1ID, in.Person2ID = in.Person2ID, in.Person1ID
		}
	}
	v.short("startDate", in.StartDate)
	v.short("endDate", in.EndDate)
	v.long("notes", in.Notes)
	return v.err()
}

// Normalize trims and validates an event input.
func (in *EventInput) Normalize() error {
	in.Type = strings.TrimSpace(in.Type)
	in.Title = strings.TrimSpace(in.Title)
	in.Date = strings.TrimSpace(in.Date)
	in.Place = strings.TrimSpace(in.Place)
	v := &validator{}
	v.check(IsEventType(in.Type), "type", "unknown event type")
	v.check(in.Title != "" || in.Type != "custom", "title", "custom events need a title")
	v.short("title", in.Title)
	v.short("date", in.Date)
	v.short("place", in.Place)
	v.long("description", in.Description)
	seen := map[int64]bool{}
	parts := in.Participants[:0]
	for _, p := range in.Participants {
		if p.PersonID <= 0 || seen[p.PersonID] {
			continue
		}
		seen[p.PersonID] = true
		p.Role = strings.TrimSpace(p.Role)
		if p.Role == "" {
			p.Role = "principal"
		}
		parts = append(parts, p)
	}
	in.Participants = parts
	if in.Type != "family" && in.Type != "historical" {
		v.check(len(in.Participants) > 0, "participants", "add at least one person to this event")
	}
	return v.err()
}

// Normalize trims and validates a note input.
func (in *NoteInput) Normalize() error {
	in.Title = strings.TrimSpace(in.Title)
	v := &validator{}
	v.check(in.Title != "" || strings.TrimSpace(in.Body) != "", "body", "a note needs a title or text")
	v.short("title", in.Title)
	v.long("body", in.Body)
	return v.err()
}

// Normalize trims and validates media metadata.
func (in *MediaInput) Normalize() error {
	in.Title = strings.TrimSpace(in.Title)
	in.TakenDate = strings.TrimSpace(in.TakenDate)
	v := &validator{}
	v.short("title", in.Title)
	v.short("takenDate", in.TakenDate)
	v.long("description", in.Description)
	in.PersonIDs = slices.Compact(slices.Sorted(slices.Values(in.PersonIDs)))
	return v.err()
}
