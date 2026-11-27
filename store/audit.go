package store

import (
	"encoding/json"
	"time"
)

// Audit actions.
const (
	ActionCreate  = "create"
	ActionUpdate  = "update"
	ActionDelete  = "delete"
	ActionImport  = "import"
	ActionBackup  = "backup"
	ActionRestore = "restore"
	ActionLogin   = "login"
)

// Entity types recorded in the audit log.
const (
	EntityPerson       = "person"
	EntityRelationship = "relationship"
	EntityEvent        = "event"
	EntityNote         = "note"
	EntityMedia        = "media"
	EntityUser         = "user"
	EntitySystem       = "system"
	EntitySetting      = "setting"
)

// FieldChange is a before/after pair for one field.
type FieldChange struct {
	From any `json:"from"`
	To   any `json:"to"`
}

// AuditEntry is one row of the change log.
type AuditEntry struct {
	ID          int64                  `json:"id"`
	CreatedAt   time.Time              `json:"createdAt"`
	User        *UserRef               `json:"user"`     // nil if the user was removed or system action
	Username    string                 `json:"username"` // snapshot at time of change
	Action      string                 `json:"action"`
	EntityType  string                 `json:"entityType"`
	EntityID    int64                  `json:"entityId"`
	EntityLabel string                 `json:"entityLabel"`
	PersonIDs   []int64                `json:"personIds"` // people this change concerns
	Changes     map[string]FieldChange `json:"changes"`
	Summary     string                 `json:"summary"`
}

// FindAudit filters the change log.
type FindAudit struct {
	EntityType string
	EntityID   *int64
	PersonID   *int64
	UserID     *int64
	BeforeID   int64 // pagination cursor
	Limit      int
}

// Change is broadcast to plugins after a committed mutation.
type Change struct {
	Action     string
	EntityType string
	EntityID   int64
	Actor      Actor
	PersonIDs  []int64
}

// Diff compares two JSON-serialisable values field by field (top level only).
// Fields listed in ignore are skipped.
func Diff(before, after any, ignore ...string) map[string]FieldChange {
	b := toMap(before)
	a := toMap(after)
	skip := map[string]bool{"createdAt": true, "updatedAt": true, "createdBy": true, "updatedBy": true, "id": true}
	for _, k := range ignore {
		skip[k] = true
	}
	out := map[string]FieldChange{}
	keys := map[string]bool{}
	for k := range b {
		keys[k] = true
	}
	for k := range a {
		keys[k] = true
	}
	for k := range keys {
		if skip[k] {
			continue
		}
		bj, _ := json.Marshal(b[k])
		aj, _ := json.Marshal(a[k])
		if string(bj) != string(aj) {
			out[k] = FieldChange{From: b[k], To: a[k]}
		}
	}
	return out
}

func toMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}
	m := map[string]any{}
	if json.Unmarshal(raw, &m) != nil {
		return map[string]any{}
	}
	return m
}
