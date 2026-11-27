// Package audit exposes the change log.
package audit

import (
	"net/http"

	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// Module is the audit module.
type Module struct{ Store *store.Store }

// Register mounts routes.
func (m *Module) Register(rt *apiutil.Router) {
	rt.Handle("GET /api/v1/audit", apiutil.Viewer, m.list)
}

// Response is a page of audit entries with a cursor for the next page.
type Response struct {
	Items      []*store.AuditEntry `json:"items"`
	NextBefore int64               `json:"nextBefore"` // 0 when there are no more entries
}

func (m *Module) list(w http.ResponseWriter, r *http.Request) error {
	f := store.FindAudit{EntityType: r.URL.Query().Get("entityType"), Limit: apiutil.QueryInt(r, "limit", 50, 1, 200)}
	var err error
	if f.EntityID, err = apiutil.QueryID(r, "entityId"); err != nil {
		return err
	}
	if f.PersonID, err = apiutil.QueryID(r, "personId"); err != nil {
		return err
	}
	if f.UserID, err = apiutil.QueryID(r, "userId"); err != nil {
		return err
	}
	if b, err := apiutil.QueryID(r, "before"); err != nil {
		return err
	} else if b != nil {
		f.BeforeID = *b
	}
	// Fetch one extra to know whether another page exists.
	limit := f.Limit
	f.Limit = limit + 1
	items, err := m.Store.ListAudit(r.Context(), f)
	if err != nil {
		return err
	}
	resp := Response{Items: items}
	if len(items) > limit {
		resp.Items = items[:limit]
		resp.NextBefore = resp.Items[limit-1].ID
	}
	return apiutil.OK(w, resp)
}
