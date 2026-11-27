package tree

import (
	"context"
	"slices"

	"github.com/novapixel1010/family-holocron/store"
)

// Graph roles.
const (
	RoleRoot       = "root"
	RoleAncestor   = "ancestor"
	RoleDescendant = "descendant"
	RolePartner    = "partner" // spouse or co-parent of someone in the tree
)

// GraphPerson is one unique person in the flat tree graph.
type GraphPerson struct {
	*Summary
	// Generation is the distance from the root along the tree direction
	// (partners share the generation of the person they are linked to).
	Generation int    `json:"generation"`
	Role       string `json:"role"`
	// HasMore is true when further generations exist beyond what was returned.
	HasMore bool `json:"hasMore"`
}

// GraphLink is one relationship between two people in the graph.
type GraphLink struct {
	ID      int64  `json:"id"`     // relationship id (unique)
	Type    string `json:"type"`   // "parent" or "spouse"
	Source  int64  `json:"source"` // parent (parent links) or first partner
	Target  int64  `json:"target"` // child (parent links) or second partner
	Subtype string `json:"subtype"`
}

// buildGraph flattens the walked tree into unique people and every relationship
// among them, adding partners and co-parents so no valid link is omitted.
func (m *Module) buildGraph(ctx context.Context, rootID int64, dir string, people map[int64]*store.Person,
	depth map[int64]int, links map[int64][]*store.Relationship) ([]GraphPerson, []GraphLink, error) {
	nodes := map[int64]*GraphPerson{}
	for id, p := range people {
		if p == nil {
			continue
		}
		role := RoleAncestor
		if dir == "descendants" {
			role = RoleDescendant
		}
		if id == rootID {
			role = RoleRoot
		}
		nodes[id] = &GraphPerson{Summary: summarize(p), Generation: depth[id], Role: role}
	}
	rels := map[int64]*store.Relationship{}
	for from, rs := range links {
		if nodes[from] == nil {
			continue
		}
		for _, r := range rs {
			if nodes[r.Person1ID] != nil && nodes[r.Person2ID] != nil {
				rels[r.ID] = r
			} else {
				nodes[from].HasMore = true // the next generation lies beyond the requested depth
			}
		}
	}
	lineage := make([]int64, 0, len(nodes))
	for id := range nodes {
		lineage = append(lineage, id)
	}
	slices.Sort(lineage)

	partners := map[int64]int{} // partner id -> generation
	addPartner := func(id int64, gen int) {
		if nodes[id] == nil {
			if g, ok := partners[id]; !ok || gen < g {
				partners[id] = gen
			}
		}
	}
	if dir == "descendants" {
		// Co-parents: every parent of a descendant's child (the root's own parents are out of scope).
		var children []int64
		for _, id := range lineage {
			if id != rootID {
				children = append(children, id)
			}
		}
		if len(children) > 0 {
			prs, err := m.Store.ListRelationships(ctx, store.FindRelationship{ChildIDs: children})
			if err != nil {
				return nil, nil, err
			}
			for _, r := range prs {
				addPartner(r.Person1ID, depth[r.Person2ID]-1)
				rels[r.ID] = r
			}
		}
		// Spouses and partners of every descendant.
		srs, err := m.Store.ListRelationships(ctx, store.FindRelationship{PersonIDs: lineage, Type: store.RelSpouse})
		if err != nil {
			return nil, nil, err
		}
		for _, r := range srs {
			if nodes[r.Person1ID] != nil {
				addPartner(r.Person2ID, depth[r.Person1ID])
			} else {
				addPartner(r.Person1ID, depth[r.Person2ID])
			}
			rels[r.ID] = r
		}
	} else {
		// Couples among the ancestors (e.g. a person's two parents).
		srs, err := m.Store.ListRelationships(ctx, store.FindRelationship{PersonIDs: lineage, Type: store.RelSpouse})
		if err != nil {
			return nil, nil, err
		}
		for _, r := range srs {
			if nodes[r.Person1ID] != nil && nodes[r.Person2ID] != nil {
				rels[r.ID] = r
			}
		}
	}
	if len(partners) > 0 {
		ids := make([]int64, 0, len(partners))
		for id := range partners {
			ids = append(ids, id)
		}
		ps, _, err := m.Store.ListPersons(ctx, store.FindPerson{IDs: ids})
		if err != nil {
			return nil, nil, err
		}
		for _, p := range ps {
			nodes[p.ID] = &GraphPerson{Summary: summarize(p), Generation: partners[p.ID], Role: RolePartner}
		}
	}

	outPeople := make([]GraphPerson, 0, len(nodes))
	for _, n := range nodes {
		outPeople = append(outPeople, *n)
	}
	slices.SortFunc(outPeople, func(a, b GraphPerson) int {
		if a.Generation != b.Generation {
			return a.Generation - b.Generation
		}
		return int(a.ID - b.ID)
	})
	outLinks := make([]GraphLink, 0, len(rels))
	for _, r := range rels {
		if nodes[r.Person1ID] == nil || nodes[r.Person2ID] == nil {
			continue // endpoint could not be loaded
		}
		outLinks = append(outLinks, GraphLink{ID: r.ID, Type: r.Type, Source: r.Person1ID, Target: r.Person2ID, Subtype: r.Subtype})
	}
	slices.SortFunc(outLinks, func(a, b GraphLink) int { return int(a.ID - b.ID) })
	return outPeople, outLinks, nil
}
