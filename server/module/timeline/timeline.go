// Package timeline implements events CRUD and the chronological timeline,
// which merges stored events with births and deaths from person records.
package timeline

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/novapixel1010/family-holocron/internal/gendate"
	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// Module is the timeline domain module.
type Module struct{ Store *store.Store }

// Register mounts routes.
func (m *Module) Register(rt *apiutil.Router) {
	rt.Handle("GET /api/v1/event-types", apiutil.Viewer, m.types)
	rt.Handle("GET /api/v1/events", apiutil.Viewer, m.list)
	rt.Handle("POST /api/v1/events", apiutil.Editor, m.create)
	rt.Handle("GET /api/v1/events/{id}", apiutil.Viewer, m.get)
	rt.Handle("PUT /api/v1/events/{id}", apiutil.Editor, m.update)
	rt.Handle("DELETE /api/v1/events/{id}", apiutil.Editor, m.delete)
	rt.Handle("GET /api/v1/timeline", apiutil.Viewer, m.timeline)
}

func (m *Module) types(w http.ResponseWriter, r *http.Request) error {
	return apiutil.OK(w, map[string]any{"eventTypes": store.AllEventTypes(), "vitalTypes": []string{"birth", "death"}})
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (m *Module) list(w http.ResponseWriter, r *http.Request) error {
	pid, err := apiutil.QueryID(r, "personId")
	if err != nil {
		return err
	}
	events, err := m.Store.ListEvents(r.Context(), store.FindEvent{PersonID: pid, Types: splitList(r.URL.Query().Get("types"))})
	if err != nil {
		return err
	}
	return apiutil.OK(w, events)
}

func (m *Module) create(w http.ResponseWriter, r *http.Request) error {
	var in store.EventInput
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	if err := checkEventDate(&in, ""); err != nil {
		return err
	}
	e, err := m.Store.CreateEvent(r.Context(), apiutil.ActorFrom(r.Context()), in)
	if err != nil {
		return err
	}
	return apiutil.Created(w, e)
}

func (m *Module) get(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	e, err := m.Store.GetEvent(r.Context(), id)
	if err == store.ErrNotFound {
		return apiutil.NotFound("Event")
	}
	if err != nil {
		return err
	}
	return apiutil.OK(w, e)
}

func (m *Module) update(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	var in store.EventInput
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	before, err := m.Store.GetEvent(r.Context(), id)
	if err == store.ErrNotFound {
		return apiutil.NotFound("Event")
	}
	if err != nil {
		return err
	}
	if err := checkEventDate(&in, before.Date); err != nil {
		return err
	}
	e, err := m.Store.UpdateEvent(r.Context(), apiutil.ActorFrom(r.Context()), id, in)
	if err == store.ErrNotFound {
		return apiutil.NotFound("Event")
	}
	if err != nil {
		return err
	}
	return apiutil.OK(w, e)
}

func (m *Module) delete(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	err = m.Store.DeleteEvent(r.Context(), apiutil.ActorFrom(r.Context()), id)
	if err == store.ErrNotFound {
		return apiutil.NotFound("Event")
	}
	if err != nil {
		return err
	}
	return apiutil.NoContent(w)
}

// Item is one entry on the timeline.
type Item struct {
	Key         string                   `json:"key"`
	Kind        string                   `json:"kind"` // "event", "birth", "death"
	Type        string                   `json:"type"`
	Title       string                   `json:"title"`
	Date        string                   `json:"date"`
	SortDate    string                   `json:"sortDate"`
	Year        int                      `json:"year"`
	Place       string                   `json:"place"`
	Description string                   `json:"description"`
	EventID     int64                    `json:"eventId,omitempty"`
	People      []store.EventParticipant `json:"people"`
	store.Attribution
}

// Response is the timeline payload.
type Response struct {
	Items     []Item `json:"items"`
	Total     int    `json:"total"`
	Truncated bool   `json:"truncated"`
}

func (m *Module) timeline(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	q := r.URL.Query()
	pid, err := apiutil.QueryID(r, "personId")
	if err != nil {
		return err
	}
	types := splitList(q.Get("types"))
	want := func(t string) bool { return len(types) == 0 || slices.Contains(types, t) }
	from, to := strings.TrimSpace(q.Get("from")), strings.TrimSpace(q.Get("to"))
	text := strings.ToLower(strings.TrimSpace(q.Get("q")))
	limit := apiutil.QueryInt(r, "limit", 2000, 1, 10000)

	var eventTypes []string
	for _, t := range types {
		if t != "birth" && t != "death" {
			eventTypes = append(eventTypes, t)
		}
	}
	items := []Item{}
	if len(types) == 0 || len(eventTypes) > 0 {
		events, err := m.Store.ListEvents(ctx, store.FindEvent{PersonID: pid, Types: eventTypes, Query: q.Get("q"), From: from, To: to})
		if err != nil {
			return err
		}
		for _, e := range events {
			items = append(items, Item{
				Key: "event-" + itoa(e.ID), Kind: "event", Type: e.Type, Title: e.Title, Date: e.Date, SortDate: e.SortDate,
				Year: gendate.Parse(e.Date).Year, Place: e.Place, Description: e.Description, EventID: e.ID,
				People: e.Participants, Attribution: e.Attribution,
			})
		}
	}
	if want("birth") || want("death") {
		find := store.FindPerson{}
		if pid != nil {
			find.IDs = []int64{*pid}
		}
		people, _, err := m.Store.ListPersons(ctx, find)
		if err != nil {
			return err
		}
		inRange := func(sort string) bool {
			if from == "" && to == "" {
				return true
			}
			if sort == "" {
				return false
			}
			return (from == "" || sort >= from) && (to == "" || sort <= to+"~")
		}
		for _, p := range people {
			ref := []store.EventParticipant{{PersonID: p.ID, Role: "principal", Name: p.DisplayName()}}
			matches := text == "" || strings.Contains(strings.ToLower(p.DisplayName()+" "+p.BirthPlace+" "+p.DeathPlace), text)
			if !matches {
				continue
			}
			if want("birth") && (p.BirthDate != "" || p.BirthPlace != "") && inRange(p.BirthSort) {
				items = append(items, Item{Key: "birth-" + itoa(p.ID), Kind: "birth", Type: "birth", Title: "Birth of " + p.DisplayName(),
					Date: p.BirthDate, SortDate: p.BirthSort, Year: gendate.Parse(p.BirthDate).Year, Place: p.BirthPlace,
					People: ref, Attribution: p.Attribution})
			}
			if want("death") && (p.DeathDate != "" || p.DeathPlace != "") && inRange(p.DeathSort) {
				items = append(items, Item{Key: "death-" + itoa(p.ID), Kind: "death", Type: "death", Title: "Death of " + p.DisplayName(),
					Date: p.DeathDate, SortDate: p.DeathSort, Year: gendate.Parse(p.DeathDate).Year, Place: p.DeathPlace,
					People: ref, Attribution: p.Attribution})
			}
		}
	}
	slices.SortStableFunc(items, func(a, b Item) int {
		switch {
		case a.SortDate == "" && b.SortDate != "":
			return 1
		case a.SortDate != "" && b.SortDate == "":
			return -1
		case a.SortDate < b.SortDate:
			return -1
		case a.SortDate > b.SortDate:
			return 1
		}
		return strings.Compare(kindOrder(a.Kind), kindOrder(b.Kind))
	})
	resp := Response{Total: len(items)}
	if len(items) > limit {
		items = items[:limit]
		resp.Truncated = true
	}
	resp.Items = items
	return apiutil.OK(w, resp)
}

func kindOrder(k string) string {
	switch k {
	case "birth":
		return "0"
	case "event":
		return "1"
	}
	return "2"
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// checkEventDate enforces the YYYY-MM-DD standard (or YYYY-MM / YYYY) on event dates,
// normalizing in place. Family milestones may be upcoming, so only they may be in the
// future. A pre-standard value (e.g. from a GEDCOM import) is accepted while unchanged.
func checkEventDate(in *store.EventInput, stored string) error {
	in.Date = strings.TrimSpace(in.Date)
	n, _, msg := gendate.CheckField(in.Date, stored, in.Type == "family", time.Now())
	if msg != "" {
		return &store.ValidationError{Message: "Please correct the date.", Fields: map[string]string{"date": msg}}
	}
	in.Date = n
	return nil
}
