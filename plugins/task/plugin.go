// Package task is the Research Tasks plugin: a shared to-do list for genealogy
// research ("find Walter's 1911 census entry") that family members can claim,
// link to people and media, and track to completion.
//
// Everything specific to the plugin lives in this folder: Go code, SQLite
// migrations (migrations/), HTTP handlers mounted under /api/v1/plugins/task/,
// and the React UI (web/). Its tables are prefixed plg_task_ and live in the main
// database, so Settings → Backup & restore covers them.
package task

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/novapixel1010/family-holocron/plugin"
	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// ID is the plugin id (also its API prefix and table prefix).
const ID = "task"

//go:embed migrations/*.sql
var migrationFS embed.FS

// Plugin implements plugin.Plugin.
type Plugin struct {
	repo *repo
}

// New returns the plugin.
func New() *Plugin { return &Plugin{} }

// Manifest implements plugin.Plugin.
func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID: ID, Name: "Research Tasks", Version: "1.1.0", DefaultEnabled: true,
		Description: "A shared research to-do list: claim tasks, link them to people and sources, and track progress.",
		Slots: []plugin.Slot{
			{Kind: plugin.SlotNav, ID: "tasks", Label: "Research tasks", Order: 40},
			{Kind: plugin.SlotPersonTab, ID: "tasks", Label: "Tasks", Order: 60},
		},
	}
}

// Init implements plugin.Plugin.
func (p *Plugin) Init(_ context.Context, h plugin.Host) error {
	p.repo = &repo{db: h.Store.PluginDB()}
	return nil
}

// Migrations implements plugin.Migrator.
func (p *Plugin) Migrations() []store.PluginMigration {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		panic(err)
	}
	var out []store.PluginMigration
	for _, e := range entries {
		v, err := strconv.Atoi(strings.SplitN(e.Name(), "_", 2)[0])
		if err != nil {
			panic("task plugin: bad migration name " + e.Name())
		}
		b, _ := migrationFS.ReadFile("migrations/" + e.Name())
		out = append(out, store.PluginMigration{Version: v, SQL: string(b)})
	}
	return out
}

// Routes implements plugin.RouteProvider (mounted under /api/v1/plugins/task).
func (p *Plugin) Routes() []plugin.Route {
	return []plugin.Route{
		{Method: http.MethodGet, Path: "/tasks", Access: apiutil.Viewer, Handler: p.list},
		{Method: http.MethodPost, Path: "/tasks", Access: apiutil.Editor, Handler: p.create},
		{Method: http.MethodGet, Path: "/tasks/{id}", Access: apiutil.Viewer, Handler: p.get},
		{Method: http.MethodPut, Path: "/tasks/{id}", Access: apiutil.Editor, Handler: p.update},
		{Method: http.MethodDelete, Path: "/tasks/{id}", Access: apiutil.Editor, Handler: p.delete},
		{Method: http.MethodPost, Path: "/tasks/{id}/claim", Access: apiutil.Editor, Handler: p.claim},
		{Method: http.MethodPost, Path: "/tasks/{id}/unclaim", Access: apiutil.Editor, Handler: p.unclaim},
		{Method: http.MethodPatch, Path: "/tasks/{id}/status", Access: apiutil.Editor, Handler: p.setStatus},
		{Method: http.MethodGet, Path: "/tasks/{id}/comments", Access: apiutil.Viewer, Handler: p.listComments},
		{Method: http.MethodPost, Path: "/tasks/{id}/comments", Access: apiutil.Editor, Handler: p.addComment},
		{Method: http.MethodPatch, Path: "/tasks/{id}/comments/{commentId}", Access: apiutil.Editor, Handler: p.editComment},
		{Method: http.MethodDelete, Path: "/tasks/{id}/comments/{commentId}", Access: apiutil.Editor, Handler: p.deleteComment},
		{Method: http.MethodGet, Path: "/tags", Access: apiutil.Viewer, Handler: p.tags},
	}
}

func notFound(err error) error {
	if err == store.ErrNotFound {
		return apiutil.NotFound("Task")
	}
	return err
}

func (p *Plugin) list(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	f := Filter{Priority: q.Get("priority"), Tag: strings.TrimSpace(q.Get("tag")), Query: q.Get("q")}
	for _, s := range strings.Split(q.Get("status"), ",") {
		if s = strings.TrimSpace(s); s != "" {
			f.Status = append(f.Status, s)
		}
	}
	var err error
	if f.PersonID, err = apiutil.QueryID(r, "personId"); err != nil {
		return err
	}
	if c := q.Get("claimedBy"); c == "me" {
		id := apiutil.UserFrom(r.Context()).ID
		f.ClaimedBy = &id
	} else if f.ClaimedBy, err = apiutil.QueryID(r, "claimedBy"); err != nil {
		return err
	}
	tasks, err := p.repo.List(r.Context(), f)
	if err != nil {
		return err
	}
	return apiutil.OK(w, tasks)
}

func (p *Plugin) get(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	t, err := p.repo.Get(r.Context(), id)
	if err != nil {
		return notFound(err)
	}
	return apiutil.OK(w, t)
}

func (p *Plugin) create(w http.ResponseWriter, r *http.Request) error {
	var in Input
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	t, err := p.repo.Create(r.Context(), apiutil.ActorFrom(r.Context()), in)
	if err != nil {
		return err
	}
	return apiutil.Created(w, t)
}

func (p *Plugin) update(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	var in Input
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	t, err := p.repo.Update(r.Context(), apiutil.ActorFrom(r.Context()), id, in)
	if err != nil {
		return notFound(err)
	}
	return apiutil.OK(w, t)
}

func (p *Plugin) delete(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := p.repo.Delete(r.Context(), apiutil.ActorFrom(r.Context()), id); err != nil {
		return notFound(err)
	}
	return apiutil.NoContent(w)
}

func (p *Plugin) claim(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		TakeOver bool `json:"takeOver"`
	}
	if r.ContentLength != 0 {
		if err := apiutil.Decode(r, &in); err != nil {
			return err
		}
	}
	t, err := p.repo.Claim(r.Context(), apiutil.ActorFrom(r.Context()), id, in.TakeOver)
	if err != nil {
		return notFound(err)
	}
	return apiutil.OK(w, t)
}

func (p *Plugin) unclaim(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	u := apiutil.UserFrom(r.Context())
	t, err := p.repo.Unclaim(r.Context(), u.Actor(), u.Role == store.RoleAdmin, id)
	if err != nil {
		return notFound(err)
	}
	return apiutil.OK(w, t)
}

func (p *Plugin) tags(w http.ResponseWriter, r *http.Request) error {
	tags, err := p.repo.Tags(r.Context())
	if err != nil {
		return err
	}
	return apiutil.OK(w, tags)
}
