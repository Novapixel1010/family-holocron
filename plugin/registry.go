package plugin

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"sync"

	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

var idRe = regexp.MustCompile(`^[a-z][a-z0-9-]{1,40}$`)

// Registry holds the compiled-in plugins.
type Registry struct {
	mu       sync.RWMutex
	plugins  []Plugin
	byID     map[string]Plugin
	enabled  map[string]bool
	forceOff map[string]bool
	store    *store.Store
}

// NewRegistry creates an empty registry. Plugins listed in disabled are forced off.
func NewRegistry(disabled ...string) *Registry {
	r := &Registry{byID: map[string]Plugin{}, enabled: map[string]bool{}, forceOff: map[string]bool{}}
	for _, id := range disabled {
		r.forceOff[id] = true
	}
	return r
}

// Register adds a plugin. It panics on invalid or duplicate ids (programming error).
func (r *Registry) Register(p Plugin) {
	m := p.Manifest()
	if !idRe.MatchString(m.ID) {
		panic(fmt.Sprintf("plugin: invalid id %q", m.ID))
	}
	if _, dup := r.byID[m.ID]; dup {
		panic(fmt.Sprintf("plugin: duplicate id %q", m.ID))
	}
	r.plugins = append(r.plugins, p)
	r.byID[m.ID] = p
}

// Init runs migrations, Init hooks and change subscriptions, and loads enabled state.
func (r *Registry) Init(ctx context.Context, host Host) error {
	r.store = host.Store
	for _, p := range r.plugins {
		id := p.Manifest().ID
		if m, ok := p.(Migrator); ok {
			if err := host.Store.ApplyPluginMigrations(ctx, id, m.Migrations()); err != nil {
				return err
			}
		}
		if et, ok := p.(EventTypeProvider); ok {
			for _, t := range et.EventTypes() {
				store.RegisterEventType(t)
			}
		}
		if err := p.Init(ctx, Host{Store: host.Store, Logger: host.Logger.With("plugin", id)}); err != nil {
			return fmt.Errorf("plugin %s: %w", id, err)
		}
		if l, ok := p.(ChangeListener); ok {
			pid := id
			host.Store.Subscribe(func(c store.Change) {
				if r.Enabled(pid) {
					l.OnChange(context.Background(), c)
				}
			})
		}
	}
	return r.Reload(ctx)
}

// Reload re-reads enabled flags from settings (e.g. after a restore).
func (r *Registry) Reload(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.plugins {
		m := p.Manifest()
		on := m.DefaultEnabled
		if v, ok, err := r.store.GetSetting(ctx, store.PluginEnabledSetting(m.ID)); err != nil {
			return err
		} else if ok {
			on, _ = strconv.ParseBool(v)
		}
		r.enabled[m.ID] = on && !r.forceOff[m.ID]
	}
	return nil
}

// Enabled reports whether plugin id is installed and enabled.
func (r *Registry) Enabled(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enabled[id]
}

// SetEnabled toggles a plugin and persists the choice.
func (r *Registry) SetEnabled(ctx context.Context, actor store.Actor, id string, on bool) error {
	if _, ok := r.byID[id]; !ok {
		return store.ErrNotFound
	}
	if on && r.forceOff[id] {
		return store.ConflictError("this plugin is disabled by the server configuration (HOLOCRON_DISABLED_PLUGINS)")
	}
	if err := r.store.SetSetting(ctx, actor, store.PluginEnabledSetting(id), strconv.FormatBool(on)); err != nil {
		return err
	}
	r.mu.Lock()
	r.enabled[id] = on
	r.mu.Unlock()
	return nil
}

// Info is the API view of a plugin.
type Info struct {
	Manifest
	Enabled bool `json:"enabled"`
	Locked  bool `json:"locked"` // disabled by server config
}

// List returns all plugins with state.
func (r *Registry) List() []Info {
	out := make([]Info, 0, len(r.plugins))
	for _, p := range r.plugins {
		m := p.Manifest()
		if m.Slots == nil {
			m.Slots = []Slot{}
		}
		out = append(out, Info{Manifest: m, Enabled: r.Enabled(m.ID), Locked: r.forceOff[m.ID]})
	}
	slices.SortFunc(out, func(a, b Info) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return out
}

// MountRoutes registers every plugin's routes under /api/v1/plugins/<id>/,
// gated so they return 404 while the plugin is disabled.
func (r *Registry) MountRoutes(rt *apiutil.Router) {
	for _, p := range r.plugins {
		rp, ok := p.(RouteProvider)
		if !ok {
			continue
		}
		id := p.Manifest().ID
		gated := rt.WithGate(func(*http.Request) error {
			if !r.Enabled(id) {
				return apiutil.NotFound("plugin")
			}
			return nil
		})
		for _, route := range rp.Routes() {
			gated.Handle(route.Method+" /api/v1/plugins/"+id+route.Path, route.Access, route.Handler)
		}
	}
}
