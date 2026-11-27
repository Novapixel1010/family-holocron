// Package plugin defines Family Holocron's extension interface.
//
// A plugin is a Go package compiled into the binary and registered in
// bin/holocron/main.go. Core v1 tables and views are never modified by plugins;
// instead a plugin may:
//
//   - own tables (prefixed plg_<id>_) via Migrator,
//   - expose API routes under /api/v1/plugins/<id>/ via RouteProvider,
//   - react to committed changes via ChangeListener,
//   - declare UI slots (nav entries, person tabs, settings sections) in its
//     Manifest, rendered by the matching web plugin in web/src/plugins/.
//
// Plugin tables live in the main SQLite file, so backups and restores include
// them automatically.
package plugin

import (
	"context"
	"log/slog"

	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// Slot kinds understood by the web client.
const (
	SlotNav             = "nav"              // sidebar entry + page under /x/<plugin>/<path>
	SlotPersonTab       = "person.tab"       // tab on the person profile
	SlotSettingsSection = "settings.section" // section on the settings page
)

// Slot declares a UI extension point the plugin fills.
type Slot struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label"`
	Path  string `json:"path,omitempty"` // for nav slots
	Order int    `json:"order"`
}

// Manifest describes a plugin.
type Manifest struct {
	ID          string `json:"id"` // lowercase, [a-z0-9-]
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Slots       []Slot `json:"slots"`
	// DefaultEnabled controls whether the plugin is on for a fresh instance.
	DefaultEnabled bool `json:"defaultEnabled"`
}

// Host is what the core hands to a plugin at Init.
type Host struct {
	Store  *store.Store
	Logger *slog.Logger
}

// Plugin is the required interface.
type Plugin interface {
	Manifest() Manifest
	Init(ctx context.Context, host Host) error
}

// Route is a plugin API route. Path is relative to /api/v1/plugins/<id>.
type Route struct {
	Method  string
	Path    string // e.g. "/summary" or "/persons/{id}/kits"
	Access  apiutil.Access
	Handler apiutil.HandlerFunc
}

// RouteProvider is implemented by plugins that expose HTTP routes.
type RouteProvider interface {
	Routes() []Route
}

// Migrator is implemented by plugins that own database tables.
type Migrator interface {
	Migrations() []store.PluginMigration
}

// ChangeListener is implemented by plugins that react to committed changes
// (e.g. clean up their own rows when a person is deleted).
type ChangeListener interface {
	OnChange(ctx context.Context, c store.Change)
}

// EventTypeProvider is implemented by plugins adding timeline event types.
type EventTypeProvider interface {
	EventTypes() []string
}
