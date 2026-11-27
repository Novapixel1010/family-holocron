// Package person implements the people, relationships and notes API.
package person

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// Module is the person domain module.
type Module struct{ Store *store.Store }

// Register mounts routes.
func (m *Module) Register(rt *apiutil.Router) {
	rt.Handle("GET /api/v1/persons", apiutil.Viewer, m.list)
	rt.Handle("POST /api/v1/persons", apiutil.Editor, m.create)
	rt.Handle("GET /api/v1/persons/{id}", apiutil.Viewer, m.get)
	rt.Handle("PUT /api/v1/persons/{id}", apiutil.Editor, m.update)
	rt.Handle("DELETE /api/v1/persons/{id}", apiutil.Editor, m.delete)
	rt.Handle("GET /api/v1/persons/{id}/family", apiutil.Viewer, m.family)

	rt.Handle("GET /api/v1/relationships", apiutil.Viewer, m.listRels)
	rt.Handle("POST /api/v1/relationships", apiutil.Editor, m.createRel)
	rt.Handle("GET /api/v1/relationships/{id}", apiutil.Viewer, m.getRel)
	rt.Handle("PUT /api/v1/relationships/{id}", apiutil.Editor, m.updateRel)
	rt.Handle("DELETE /api/v1/relationships/{id}", apiutil.Editor, m.deleteRel)

	rt.Handle("GET /api/v1/persons/{id}/notes", apiutil.Viewer, m.listNotes)
	rt.Handle("POST /api/v1/persons/{id}/notes", apiutil.Editor, m.createNote)
	rt.Handle("PUT /api/v1/notes/{id}", apiutil.Editor, m.updateNote)
	rt.Handle("DELETE /api/v1/notes/{id}", apiutil.Editor, m.deleteNote)
}

func (m *Module) list(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	find := store.FindPerson{
		Query:  q.Get("q"),
		Sort:   store.PersonSort(q.Get("sort")),
		Limit:  apiutil.QueryInt(r, "limit", 50, 1, 1000),
		Offset: apiutil.QueryInt(r, "offset", 0, 0, 1<<30),
	}
	if ids := q.Get("ids"); ids != "" {
		find.IDs = []int64{}
		for _, part := range strings.Split(ids, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err != nil || id <= 0 {
				return apiutil.BadRequest("invalid ids")
			}
			find.IDs = append(find.IDs, id)
		}
	}
	items, total, err := m.Store.ListPersons(r.Context(), find)
	if err != nil {
		return err
	}
	return apiutil.OK(w, apiutil.Page[*store.Person]{Items: items, Total: total})
}

func (m *Module) create(w http.ResponseWriter, r *http.Request) error {
	var in store.PersonInput
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	if err := checkVitalDates(&in, nil, time.Now()); err != nil {
		return err
	}
	p, err := m.Store.CreatePerson(r.Context(), apiutil.ActorFrom(r.Context()), in)
	if err != nil {
		return err
	}
	return apiutil.Created(w, p)
}

func (m *Module) get(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	p, err := m.Store.GetPerson(r.Context(), id)
	if err != nil {
		return notFound(err, "Person")
	}
	return apiutil.OK(w, p)
}

func notFound(err error, what string) error {
	if err == store.ErrNotFound {
		return apiutil.NotFound(what)
	}
	return err
}

func (m *Module) update(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	var in store.PersonInput
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	before, err := m.Store.GetPerson(r.Context(), id)
	if err != nil {
		return notFound(err, "Person")
	}
	if err := checkVitalDates(&in, before, time.Now()); err != nil {
		return err
	}
	p, err := m.Store.UpdatePerson(r.Context(), apiutil.ActorFrom(r.Context()), id, in)
	if err != nil {
		return notFound(err, "Person")
	}
	return apiutil.OK(w, p)
}

func (m *Module) delete(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := m.Store.DeletePerson(r.Context(), apiutil.ActorFrom(r.Context()), id); err != nil {
		return notFound(err, "Person")
	}
	return apiutil.NoContent(w)
}

// Relative is a person together with the relationship that links them.
type Relative struct {
	Person       *store.Person       `json:"person"`
	Relationship *store.Relationship `json:"relationship,omitempty"`
}

// Family is the immediate family of a person.
type Family struct {
	Parents  []Relative `json:"parents"`
	Spouses  []Relative `json:"spouses"`
	Children []Relative `json:"children"`
	Siblings []Relative `json:"siblings"`
}

func (m *Module) family(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	ctx := r.Context()
	if _, err := m.Store.GetPerson(ctx, id); err != nil {
		return notFound(err, "Person")
	}
	rels, err := m.Store.ListRelationships(ctx, store.FindRelationship{PersonID: &id})
	if err != nil {
		return err
	}
	fam := Family{Parents: []Relative{}, Spouses: []Relative{}, Children: []Relative{}, Siblings: []Relative{}}
	type link struct {
		bucket *[]Relative
		other  int64
		rel    *store.Relationship
	}
	var links []link
	var parentIDs []int64
	for _, rel := range rels {
		switch {
		case rel.Type == store.RelParent && rel.Person2ID == id:
			links = append(links, link{&fam.Parents, rel.Person1ID, rel})
			parentIDs = append(parentIDs, rel.Person1ID)
		case rel.Type == store.RelParent && rel.Person1ID == id:
			links = append(links, link{&fam.Children, rel.Person2ID, rel})
		case rel.Type == store.RelSpouse:
			other := rel.Person1ID
			if other == id {
				other = rel.Person2ID
			}
			links = append(links, link{&fam.Spouses, other, rel})
		}
	}
	// Siblings: other children of any parent.
	sibIDs := []int64{}
	if len(parentIDs) > 0 {
		sib, err := m.Store.ListRelationships(ctx, store.FindRelationship{ParentIDs: parentIDs})
		if err != nil {
			return err
		}
		for _, s := range sib {
			if s.Person2ID != id && !slices.Contains(sibIDs, s.Person2ID) {
				sibIDs = append(sibIDs, s.Person2ID)
			}
		}
	}
	ids := append([]int64{}, sibIDs...)
	for _, l := range links {
		ids = append(ids, l.other)
	}
	people, _, err := m.Store.ListPersons(ctx, store.FindPerson{IDs: ids, Sort: store.SortByBirth})
	if err != nil {
		return err
	}
	byID := map[int64]*store.Person{}
	for _, p := range people {
		byID[p.ID] = p
	}
	for _, l := range links {
		if p := byID[l.other]; p != nil {
			*l.bucket = append(*l.bucket, Relative{Person: p, Relationship: l.rel})
		}
	}
	for _, p := range people {
		if slices.Contains(sibIDs, p.ID) {
			fam.Siblings = append(fam.Siblings, Relative{Person: p})
		}
	}
	return apiutil.OK(w, fam)
}

func (m *Module) listRels(w http.ResponseWriter, r *http.Request) error {
	pid, err := apiutil.QueryID(r, "personId")
	if err != nil {
		return err
	}
	rels, err := m.Store.ListRelationships(r.Context(), store.FindRelationship{PersonID: pid, Type: r.URL.Query().Get("type")})
	if err != nil {
		return err
	}
	return apiutil.OK(w, rels)
}

func (m *Module) createRel(w http.ResponseWriter, r *http.Request) error {
	var in store.RelationshipInput
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	rel, err := m.Store.CreateRelationship(r.Context(), apiutil.ActorFrom(r.Context()), in)
	if err != nil {
		return err
	}
	return apiutil.Created(w, rel)
}

func (m *Module) getRel(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	rel, err := m.Store.GetRelationship(r.Context(), id)
	if err != nil {
		return notFound(err, "Relationship")
	}
	return apiutil.OK(w, rel)
}

func (m *Module) updateRel(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	var in store.RelationshipInput
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	rel, err := m.Store.UpdateRelationship(r.Context(), apiutil.ActorFrom(r.Context()), id, in)
	if err != nil {
		return notFound(err, "Relationship")
	}
	return apiutil.OK(w, rel)
}

func (m *Module) deleteRel(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := m.Store.DeleteRelationship(r.Context(), apiutil.ActorFrom(r.Context()), id); err != nil {
		return notFound(err, "Relationship")
	}
	return apiutil.NoContent(w)
}

func (m *Module) listNotes(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	if _, err := m.Store.GetPerson(r.Context(), id); err != nil {
		return notFound(err, "Person")
	}
	notes, err := m.Store.ListNotes(r.Context(), id)
	if err != nil {
		return err
	}
	return apiutil.OK(w, notes)
}

func (m *Module) createNote(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	var in store.NoteInput
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	n, err := m.Store.CreateNote(r.Context(), apiutil.ActorFrom(r.Context()), id, in)
	if err != nil {
		return notFound(err, "Person")
	}
	return apiutil.Created(w, n)
}

func (m *Module) updateNote(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	var in store.NoteInput
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	n, err := m.Store.UpdateNote(r.Context(), apiutil.ActorFrom(r.Context()), id, in)
	if err != nil {
		return notFound(err, "Note")
	}
	return apiutil.OK(w, n)
}

func (m *Module) deleteNote(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := m.Store.DeleteNote(r.Context(), apiutil.ActorFrom(r.Context()), id); err != nil {
		return notFound(err, "Note")
	}
	return apiutil.NoContent(w)
}
