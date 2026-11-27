// Package system exposes health, instance status, settings, stats and plugin state.
package system

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/novapixel1010/family-holocron/internal/version"
	"github.com/novapixel1010/family-holocron/plugin"
	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// Module is the system module.
type Module struct {
	Store         *store.Store
	Plugins       *plugin.Registry
	SchemaVersion func() int
	Started       time.Time
}

// Register mounts routes.
func (m *Module) Register(rt *apiutil.Router) {
	rt.Handle("GET /healthz", apiutil.Public, m.health)
	rt.Handle("GET /api/v1/status", apiutil.Public, m.status)
	rt.Handle("GET /api/v1/stats", apiutil.Viewer, m.stats)
	rt.Handle("GET /api/v1/settings", apiutil.Viewer, m.getSettings)
	rt.Handle("PUT /api/v1/settings", apiutil.Admin, m.putSettings)
	rt.Handle("GET /api/v1/plugins", apiutil.Viewer, m.plugins)
	rt.Handle("PUT /api/v1/plugins/{id}", apiutil.Admin, m.setPlugin)
}

func (m *Module) health(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := m.Store.Ping(ctx); err != nil {
		return apiutil.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "error": "database unavailable"})
	}
	return apiutil.OK(w, map[string]string{"status": "ok"})
}

func (m *Module) instanceName(ctx context.Context) string {
	if v, ok, _ := m.Store.GetSetting(ctx, store.SettingInstanceName); ok && v != "" {
		return v
	}
	return "Family Holocron"
}

func (m *Module) status(w http.ResponseWriter, r *http.Request) error {
	n, err := m.Store.CountUsers(r.Context())
	if err != nil {
		return err
	}
	return apiutil.OK(w, map[string]any{
		"version": version.Version, "schemaVersion": m.SchemaVersion(), "setupRequired": n == 0,
		"instanceName": m.instanceName(r.Context()), "signedIn": apiutil.UserFrom(r.Context()) != nil,
	})
}

func (m *Module) stats(w http.ResponseWriter, r *http.Request) error {
	s, err := m.Store.Stats(r.Context())
	if err != nil {
		return err
	}
	return apiutil.OK(w, s)
}

// Settings are instance-wide settings editable by admins.
type Settings struct {
	InstanceName string `json:"instanceName"`
}

func (m *Module) getSettings(w http.ResponseWriter, r *http.Request) error {
	return apiutil.OK(w, Settings{InstanceName: m.instanceName(r.Context())})
}

func (m *Module) putSettings(w http.ResponseWriter, r *http.Request) error {
	var in Settings
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	in.InstanceName = strings.TrimSpace(in.InstanceName)
	if in.InstanceName == "" || len(in.InstanceName) > 80 {
		return store.Invalid("instanceName", "must be 1–80 characters")
	}
	if err := m.Store.SetSetting(r.Context(), apiutil.ActorFrom(r.Context()), store.SettingInstanceName, in.InstanceName); err != nil {
		return err
	}
	return apiutil.OK(w, in)
}

func (m *Module) plugins(w http.ResponseWriter, r *http.Request) error {
	return apiutil.OK(w, m.Plugins.List())
}

func (m *Module) setPlugin(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	err := m.Plugins.SetEnabled(r.Context(), apiutil.ActorFrom(r.Context()), r.PathValue("id"), in.Enabled)
	if err == store.ErrNotFound {
		return apiutil.NotFound("Plugin")
	}
	if err != nil {
		return err
	}
	return apiutil.OK(w, m.Plugins.List())
}
