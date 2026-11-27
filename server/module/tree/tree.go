// Package tree assembles ancestor (pedigree) and descendant trees for the tree view.
package tree

import (
	"context"
	"net/http"
	"slices"

	"github.com/novapixel1010/family-holocron/internal/gendate"
	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// Module is the tree domain module.
type Module struct{ Store *store.Store }

// Register mounts routes.
func (m *Module) Register(rt *apiutil.Router) {
	rt.Handle("GET /api/v1/tree/{id}", apiutil.Viewer, m.get)
}

// Summary is the compact person view shown on tree nodes.
type Summary struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	GivenName       string `json:"givenName"`
	Surname         string `json:"surname"`
	Sex             string `json:"sex"`
	BirthDate       string `json:"birthDate"`
	DeathDate       string `json:"deathDate"`
	BirthYear       int    `json:"birthYear"`
	DeathYear       int    `json:"deathYear"`
	Living          bool   `json:"living"`
	PortraitMediaID *int64 `json:"portraitMediaId"`
}

func summarize(p *store.Person) *Summary {
	return &Summary{ID: p.ID, Name: p.DisplayName(), GivenName: p.GivenName, Surname: p.Surname, Sex: p.Sex,
		BirthDate: p.BirthDate, DeathDate: p.DeathDate, BirthYear: gendate.Parse(p.BirthDate).Year,
		DeathYear: gendate.Parse(p.DeathDate).Year, Living: p.IsLiving(), PortraitMediaID: p.PortraitMediaID}
}

// Node is one person in the tree. For ancestor trees Children holds the person's
// parents; for descendant trees it holds their children.
type Node struct {
	Person   *Summary   `json:"person"`
	Subtype  string     `json:"subtype,omitempty"` // relationship subtype to the node below/above
	Spouses  []*Summary `json:"spouses,omitempty"` // descendant view only
	Children []*Node    `json:"children"`
	// HasMore is true when the node has further generations beyond the requested depth.
	HasMore bool `json:"hasMore"`
}

// Response is the tree payload.
type Response struct {
	Direction   string `json:"direction"`
	Generations int    `json:"generations"`
	Root        *Node  `json:"root"`
	Count       int    `json:"count"`
}

func (m *Module) get(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	dir := r.URL.Query().Get("direction")
	if dir == "" {
		dir = "ancestors"
	}
	if dir != "ancestors" && dir != "descendants" {
		return apiutil.BadRequest("direction must be ancestors or descendants")
	}
	gens := apiutil.QueryInt(r, "generations", 4, 1, 12)
	resp, err := m.Build(r.Context(), id, dir, gens)
	if err == store.ErrNotFound {
		return apiutil.NotFound("Person")
	}
	if err != nil {
		return err
	}
	return apiutil.OK(w, resp)
}

// Build assembles a tree of the given direction and depth.
func (m *Module) Build(ctx context.Context, rootID int64, dir string, gens int) (*Response, error) {
	root, err := m.Store.GetPerson(ctx, rootID)
	if err != nil {
		return nil, err
	}
	people := map[int64]*store.Person{rootID: root}
	// links[id] = relationships from id to the next generation.
	links := map[int64][]*store.Relationship{}
	frontier := []int64{rootID}
	for g := 0; g <= gens && len(frontier) > 0; g++ {
		var rels []*store.Relationship
		if dir == "ancestors" {
			rels, err = m.Store.ListRelationships(ctx, store.FindRelationship{ChildIDs: frontier})
		} else {
			rels, err = m.Store.ListRelationships(ctx, store.FindRelationship{ParentIDs: frontier})
		}
		if err != nil {
			return nil, err
		}
		var next []int64
		for _, rel := range rels {
			from, to := rel.Person1ID, rel.Person2ID // parent → child
			if dir == "ancestors" {
				from, to = rel.Person2ID, rel.Person1ID
			}
			links[from] = append(links[from], rel)
			if _, seen := people[to]; !seen && g < gens {
				next = append(next, to)
				people[to] = nil
			}
		}
		if g == gens {
			break
		}
		if len(next) > 0 {
			ps, _, err := m.Store.ListPersons(ctx, store.FindPerson{IDs: next})
			if err != nil {
				return nil, err
			}
			for _, p := range ps {
				people[p.ID] = p
			}
		}
		frontier = next
	}
	spouses := map[int64][]*Summary{}
	if dir == "descendants" {
		for id, p := range people {
			if p == nil {
				continue
			}
			rels, err := m.Store.ListRelationships(ctx, store.FindRelationship{PersonID: &id, Type: store.RelSpouse})
			if err != nil {
				return nil, err
			}
			var ids []int64
			for _, rel := range rels {
				other := rel.Person1ID
				if other == id {
					other = rel.Person2ID
				}
				ids = append(ids, other)
			}
			if len(ids) > 0 {
				ps, _, err := m.Store.ListPersons(ctx, store.FindPerson{IDs: ids})
				if err != nil {
					return nil, err
				}
				for _, sp := range ps {
					spouses[id] = append(spouses[id], summarize(sp))
				}
			}
		}
	}
	count := 0
	var build func(id int64, depth int, path map[int64]bool, subtype string) *Node
	build = func(id int64, depth int, path map[int64]bool, subtype string) *Node {
		p := people[id]
		if p == nil {
			return nil
		}
		count++
		n := &Node{Person: summarize(p), Subtype: subtype, Spouses: spouses[id], Children: []*Node{}}
		if depth == gens {
			n.HasMore = len(links[id]) > 0
			return n
		}
		path[id] = true
		defer delete(path, id)
		rels := slices.Clone(links[id])
		slices.SortStableFunc(rels, func(a, b *store.Relationship) int { return rank(people, a, dir) - rank(people, b, dir) })
		for _, rel := range rels {
			next := rel.Person2ID
			if dir == "ancestors" {
				next = rel.Person1ID
			}
			if path[next] {
				continue // defensive: never loop
			}
			if c := build(next, depth+1, path, rel.Subtype); c != nil {
				n.Children = append(n.Children, c)
			}
		}
		return n
	}
	tree := build(rootID, 0, map[int64]bool{}, "")
	return &Response{Direction: dir, Generations: gens, Root: tree, Count: count}, nil
}

// rank orders parents father-first and children by birth.
func rank(people map[int64]*store.Person, rel *store.Relationship, dir string) int {
	if dir == "ancestors" {
		p := people[rel.Person1ID]
		if p == nil {
			return 9
		}
		switch p.Sex {
		case store.SexMale:
			return 0
		case store.SexFemale:
			return 1
		}
		return 2
	}
	p := people[rel.Person2ID]
	if p == nil || p.BirthSort == "" {
		return 1 << 30
	}
	return gendate.Parse(p.BirthDate).Year*400 + gendate.Parse(p.BirthDate).Month*32 + gendate.Parse(p.BirthDate).Day
}
