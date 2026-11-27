// Package store defines Family Holocron's domain models and the persistence
// Driver interface. The SQLite implementation lives in store/db/sqlite.
//
// Every mutating Driver method takes an Actor; implementations must record it in
// the record's attribution columns AND write an audit log entry in the same
// transaction.
package store

import (
	"context"
	"database/sql"
	"sync"
)

// Driver is the persistence interface implemented by database backends.
type Driver interface {
	Close() error
	Ping(ctx context.Context) error
	SchemaVersion(ctx context.Context) (int, error)
	// SetChangeListener registers a callback invoked after each committed mutation.
	SetChangeListener(fn func(Change))

	// Users & sessions.
	CountUsers(ctx context.Context) (int, error)
	CreateFirstAdmin(ctx context.Context, u *User) (*User, error)
	CreateUser(ctx context.Context, actor Actor, u *User) (*User, error)
	GetUser(ctx context.Context, id int64) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	ListUsers(ctx context.Context) ([]*User, error)
	UpdateUser(ctx context.Context, actor Actor, id int64, upd UpdateUser) (*User, error)
	CreateSession(ctx context.Context, s *Session) error
	GetSession(ctx context.Context, tokenHash string) (*Session, *User, error)
	TouchSession(ctx context.Context, tokenHash string) error
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteUserSessions(ctx context.Context, userID int64, exceptTokenHash string) error
	DeleteExpiredSessions(ctx context.Context) error

	// People.
	CreatePerson(ctx context.Context, actor Actor, in PersonInput) (*Person, error)
	GetPerson(ctx context.Context, id int64) (*Person, error)
	ListPersons(ctx context.Context, find FindPerson) ([]*Person, int, error)
	UpdatePerson(ctx context.Context, actor Actor, id int64, in PersonInput) (*Person, error)
	DeletePerson(ctx context.Context, actor Actor, id int64) error

	// Relationships.
	CreateRelationship(ctx context.Context, actor Actor, in RelationshipInput) (*Relationship, error)
	GetRelationship(ctx context.Context, id int64) (*Relationship, error)
	ListRelationships(ctx context.Context, find FindRelationship) ([]*Relationship, error)
	UpdateRelationship(ctx context.Context, actor Actor, id int64, in RelationshipInput) (*Relationship, error)
	DeleteRelationship(ctx context.Context, actor Actor, id int64) error

	// Events.
	CreateEvent(ctx context.Context, actor Actor, in EventInput) (*Event, error)
	GetEvent(ctx context.Context, id int64) (*Event, error)
	ListEvents(ctx context.Context, find FindEvent) ([]*Event, error)
	UpdateEvent(ctx context.Context, actor Actor, id int64, in EventInput) (*Event, error)
	DeleteEvent(ctx context.Context, actor Actor, id int64) error

	// Notes.
	CreateNote(ctx context.Context, actor Actor, personID int64, in NoteInput) (*Note, error)
	GetNote(ctx context.Context, id int64) (*Note, error)
	ListNotes(ctx context.Context, personID int64) ([]*Note, error)
	UpdateNote(ctx context.Context, actor Actor, id int64, in NoteInput) (*Note, error)
	DeleteNote(ctx context.Context, actor Actor, id int64) error

	// Media metadata (file bytes are handled by the media module).
	CreateMedia(ctx context.Context, actor Actor, m *Media) (*Media, error)
	GetMedia(ctx context.Context, id int64) (*Media, error)
	ListMedia(ctx context.Context, find FindMedia) ([]*Media, int, error)
	UpdateMedia(ctx context.Context, actor Actor, id int64, in MediaInput) (*Media, error)
	DeleteMedia(ctx context.Context, actor Actor, id int64) error

	// Biography templates (admin-managed Markdown for the About section).
	ListBioTemplates(ctx context.Context) ([]*BioTemplate, error)
	GetBioTemplate(ctx context.Context, id int64) (*BioTemplate, error)
	CreateBioTemplate(ctx context.Context, actor Actor, in BioTemplateInput) (*BioTemplate, error)
	UpdateBioTemplate(ctx context.Context, actor Actor, id int64, in BioTemplateInput) (*BioTemplate, error)
	DeleteBioTemplate(ctx context.Context, actor Actor, id int64) error
	// SeedBioTemplates inserts the starter template once per database: only when the
	// table is empty and it has never been seeded, so deleting every template sticks.
	SeedBioTemplates(ctx context.Context) error

	// Audit log.
	ListAudit(ctx context.Context, find FindAudit) ([]*AuditEntry, error)
	RecordAudit(ctx context.Context, actor Actor, e AuditEntry) error

	// Settings and plugin storage.
	GetSetting(ctx context.Context, name string) (string, bool, error)
	SetSetting(ctx context.Context, actor Actor, name, value string) error
	PluginGet(ctx context.Context, pluginID, key string) (string, bool, error)
	PluginSet(ctx context.Context, pluginID, key, value string) error
	PluginDelete(ctx context.Context, pluginID, key string) error
	ApplyPluginMigrations(ctx context.Context, pluginID string, migrations []PluginMigration) error
	PluginDB() PluginDB

	// Bulk import & maintenance.
	Import(ctx context.Context, actor Actor, batch *ImportBatch) (*ImportResult, error)
	Stats(ctx context.Context) (*Stats, error)
	// SnapshotTo writes a consistent copy of the database to path.
	SnapshotTo(ctx context.Context, path string) error
	// RestoreFrom replaces all data with the contents of the database at path,
	// atomically. Sessions are dropped except keep (if it still maps to the same user).
	RestoreFrom(ctx context.Context, path string, keep KeepSession) error
}

// KeepSession identifies the session to preserve across a restore.
type KeepSession struct {
	TokenHash string
	UserID    int64
	Username  string
}

// PluginMigration is a plugin-owned schema migration.
type PluginMigration struct {
	Version int
	SQL     string
}

// Stats are instance-wide counts.
type Stats struct {
	Persons       int `json:"persons"`
	Relationships int `json:"relationships"`
	Events        int `json:"events"`
	Notes         int `json:"notes"`
	Media         int `json:"media"`
	Users         int `json:"users"`
	Surnames      int `json:"surnames"`
}

// ImportBatch is a self-contained set of records referenced by local keys
// (e.g. GEDCOM xrefs). It is written in a single transaction.
type ImportBatch struct {
	Source        string
	Persons       []ImportPerson
	Relationships []ImportRelationship
	Events        []ImportEvent
	Notes         []ImportNote
}

// ImportPerson is a person keyed by a local reference.
type ImportPerson struct {
	Ref   string
	Input PersonInput
}

// ImportRelationship references persons by local key.
type ImportRelationship struct {
	Type, Person1Ref, Person2Ref, Subtype, StartDate, EndDate string
}

// ImportEvent references participants by local key.
type ImportEvent struct {
	Input           EventInput // Participants ignored; use Refs
	ParticipantRefs []string
}

// ImportNote attaches a note to a person by local key.
type ImportNote struct {
	PersonRef string
	Input     NoteInput
}

// ImportResult counts what was created.
type ImportResult struct {
	Persons       int      `json:"persons"`
	Relationships int      `json:"relationships"`
	Events        int      `json:"events"`
	Notes         int      `json:"notes"`
	Warnings      []string `json:"warnings"`
}

// Store is the façade used by the server: the Driver plus change fan-out to subscribers.
type Store struct {
	Driver
	mu   sync.RWMutex
	subs []func(Change)
}

// New wraps a driver.
func New(d Driver) *Store {
	s := &Store{Driver: d}
	d.SetChangeListener(s.dispatch)
	return s
}

// Subscribe registers fn to be called after every committed mutation.
func (s *Store) Subscribe(fn func(Change)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs = append(s.subs, fn)
}

func (s *Store) dispatch(c Change) {
	s.mu.RLock()
	subs := append([]func(Change){}, s.subs...)
	s.mu.RUnlock()
	for _, fn := range subs {
		fn(c)
	}
}

// PluginDB gives compiled-in plugins SQL access for their own plg_<id>_* tables.
// Plugins may read core tables (e.g. to join user names) but must only write to
// their own tables, and only through WriteTx so changes are audited atomically.
type PluginDB interface {
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
	// WriteTx runs fn in a write transaction. Entries passed to audit are written
	// to the change log in the same transaction and broadcast after commit.
	WriteTx(ctx context.Context, actor Actor, fn func(tx *sql.Tx, audit func(AuditEntry) error) error) error
}
