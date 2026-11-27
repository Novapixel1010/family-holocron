// Package biotemplate exposes the admin-managed Markdown templates used when
// writing a person's About / biography section.
package biotemplate

import (
	"net/http"

	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// Module is the biography-template module.
type Module struct{ Store *store.Store }

// Register mounts routes: everyone signed in can list; only admins can change.
func (m *Module) Register(rt *apiutil.Router) {
	rt.Handle("GET /api/v1/bio-templates", apiutil.Viewer, m.list)
	rt.Handle("POST /api/v1/bio-templates", apiutil.Admin, m.create)
	rt.Handle("PUT /api/v1/bio-templates/{id}", apiutil.Admin, m.update)
	rt.Handle("DELETE /api/v1/bio-templates/{id}", apiutil.Admin, m.delete)
}

func notFound(err error) error {
	if err == store.ErrNotFound {
		return apiutil.NotFound("Template")
	}
	return err
}

func (m *Module) list(w http.ResponseWriter, r *http.Request) error {
	items, err := m.Store.ListBioTemplates(r.Context())
	if err != nil {
		return err
	}
	return apiutil.OK(w, items)
}

func (m *Module) create(w http.ResponseWriter, r *http.Request) error {
	var in store.BioTemplateInput
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	t, err := m.Store.CreateBioTemplate(r.Context(), apiutil.ActorFrom(r.Context()), in)
	if err != nil {
		return err
	}
	return apiutil.Created(w, t)
}

func (m *Module) update(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	var in store.BioTemplateInput
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	t, err := m.Store.UpdateBioTemplate(r.Context(), apiutil.ActorFrom(r.Context()), id, in)
	if err != nil {
		return notFound(err)
	}
	return apiutil.OK(w, t)
}

func (m *Module) delete(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := m.Store.DeleteBioTemplate(r.Context(), apiutil.ActorFrom(r.Context()), id); err != nil {
		return notFound(err)
	}
	return apiutil.NoContent(w)
}
