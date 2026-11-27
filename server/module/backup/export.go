package backup

import (
	"context"
	"time"

	"github.com/novapixel1010/family-holocron/internal/version"
	"github.com/novapixel1010/family-holocron/store"
)

// ExportUser is the non-secret part of a user account.
type ExportUser struct {
	ID          int64      `json:"id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"displayName"`
	Role        store.Role `json:"role"`
}

// Export is the portable JSON representation of all genealogical records.
// It is readable without SQLite and is included in every backup archive.
type Export struct {
	Format        int                   `json:"format"`
	AppVersion    string                `json:"appVersion"`
	ExportedAt    time.Time             `json:"exportedAt"`
	Users         []ExportUser          `json:"users"`
	Persons       []*store.Person       `json:"persons"`
	Relationships []*store.Relationship `json:"relationships"`
	Events        []*store.Event        `json:"events"`
	Notes         []*store.Note         `json:"notes"`
	Media         []*store.Media        `json:"media"`
}

// BuildExport collects every record from the store.
func BuildExport(ctx context.Context, s *store.Store) (*Export, error) {
	ex := &Export{Format: 1, AppVersion: version.Version, ExportedAt: time.Now().UTC(), Notes: []*store.Note{}, Users: []ExportUser{}}
	users, err := s.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		ex.Users = append(ex.Users, ExportUser{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Role: u.Role})
	}
	if ex.Persons, _, err = s.ListPersons(ctx, store.FindPerson{}); err != nil {
		return nil, err
	}
	if ex.Relationships, err = s.ListRelationships(ctx, store.FindRelationship{}); err != nil {
		return nil, err
	}
	if ex.Events, err = s.ListEvents(ctx, store.FindEvent{}); err != nil {
		return nil, err
	}
	for _, p := range ex.Persons {
		notes, err := s.ListNotes(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		ex.Notes = append(ex.Notes, notes...)
	}
	if ex.Media, _, err = s.ListMedia(ctx, store.FindMedia{}); err != nil {
		return nil, err
	}
	return ex, nil
}
