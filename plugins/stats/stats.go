// Package stats is a built-in example plugin. It contributes a "Statistics"
// navigation page backed by its own API route, and keeps a per-instance
// change counter in plugin storage via a ChangeListener — demonstrating every
// extension point without touching core tables.
package stats

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/novapixel1010/family-holocron/internal/gendate"
	"github.com/novapixel1010/family-holocron/plugin"
	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// ID is the plugin id.
const ID = "stats"

// Plugin implements plugin.Plugin.
type Plugin struct {
	store  *store.Store
	logger *slog.Logger
}

// New returns the plugin.
func New() *Plugin { return &Plugin{} }

// Manifest implements plugin.Plugin.
func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID: ID, Name: "Family statistics", Version: "1.0.0", DefaultEnabled: true,
		Description: "Surname frequencies, lifespans and births by century.",
		Slots:       []plugin.Slot{{Kind: plugin.SlotNav, ID: "stats", Label: "Statistics", Path: "", Order: 50}},
	}
}

// Init implements plugin.Plugin.
func (p *Plugin) Init(_ context.Context, h plugin.Host) error {
	p.store, p.logger = h.Store, h.Logger
	return nil
}

// OnChange counts committed changes (stored in plugin_kv).
func (p *Plugin) OnChange(ctx context.Context, c store.Change) {
	v, _, err := p.store.PluginGet(ctx, ID, "changes")
	if err != nil {
		return
	}
	n, _ := strconv.Atoi(v)
	if err := p.store.PluginSet(ctx, ID, "changes", strconv.Itoa(n+1)); err != nil {
		p.logger.Warn("could not update change counter", "err", err)
	}
}

// Routes implements plugin.RouteProvider.
func (p *Plugin) Routes() []plugin.Route {
	return []plugin.Route{{Method: http.MethodGet, Path: "/summary", Access: apiutil.Viewer, Handler: p.summary}}
}

// Count is a labelled number.
type Count struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// Summary is the statistics payload.
type Summary struct {
	People          int     `json:"people"`
	Living          int     `json:"living"`
	Deceased        int     `json:"deceased"`
	BySex           []Count `json:"bySex"`
	TopSurnames     []Count `json:"topSurnames"`
	BirthsByCentury []Count `json:"birthsByCentury"`
	AverageLifespan float64 `json:"averageLifespan"`
	LifespanSample  int     `json:"lifespanSample"`
	EarliestBirth   string  `json:"earliestBirth"`
	ChangesRecorded int     `json:"changesRecorded"`
}

func (p *Plugin) summary(w http.ResponseWriter, r *http.Request) error {
	people, _, err := p.store.ListPersons(r.Context(), store.FindPerson{})
	if err != nil {
		return err
	}
	s := Summary{BySex: []Count{}, TopSurnames: []Count{}, BirthsByCentury: []Count{}}
	sex := map[string]int{}
	surnames := map[string]int{}
	display := map[string]string{}
	centuries := map[int]int{}
	var lifeTotal int
	earliest := ""
	for _, per := range people {
		s.People++
		if per.IsLiving() {
			s.Living++
		} else {
			s.Deceased++
		}
		sex[per.Sex]++
		if per.Surname != "" {
			k := strings.ToLower(per.Surname)
			surnames[k]++
			display[k] = per.Surname
		}
		b, d := gendate.Parse(per.BirthDate), gendate.Parse(per.DeathDate)
		if b.Valid() {
			centuries[(b.Year-1)/100+1]++
			if earliest == "" || per.BirthSort < earliest {
				earliest = per.BirthSort
				s.EarliestBirth = per.BirthDate
			}
		}
		if b.Valid() && d.Valid() && d.Year >= b.Year && d.Year-b.Year < 130 {
			lifeTotal += d.Year - b.Year
			s.LifespanSample++
		}
	}
	labels := map[string]string{"M": "Male", "F": "Female", "X": "Other", "U": "Unknown"}
	for _, k := range []string{"F", "M", "X", "U"} {
		if sex[k] > 0 {
			s.BySex = append(s.BySex, Count{labels[k], sex[k]})
		}
	}
	for k, n := range surnames {
		s.TopSurnames = append(s.TopSurnames, Count{display[k], n})
	}
	sort.Slice(s.TopSurnames, func(i, j int) bool {
		if s.TopSurnames[i].Count != s.TopSurnames[j].Count {
			return s.TopSurnames[i].Count > s.TopSurnames[j].Count
		}
		return s.TopSurnames[i].Label < s.TopSurnames[j].Label
	})
	if len(s.TopSurnames) > 10 {
		s.TopSurnames = s.TopSurnames[:10]
	}
	var cs []int
	for c := range centuries {
		cs = append(cs, c)
	}
	sort.Ints(cs)
	for _, c := range cs {
		s.BirthsByCentury = append(s.BirthsByCentury, Count{ordinal(c) + " century", centuries[c]})
	}
	if s.LifespanSample > 0 {
		s.AverageLifespan = float64(lifeTotal) / float64(s.LifespanSample)
	}
	if v, ok, _ := p.store.PluginGet(r.Context(), ID, "changes"); ok {
		s.ChangesRecorded, _ = strconv.Atoi(v)
	}
	return apiutil.OK(w, s)
}

func ordinal(n int) string {
	suffix := "th"
	switch {
	case n%100 >= 11 && n%100 <= 13:
	case n%10 == 1:
		suffix = "st"
	case n%10 == 2:
		suffix = "nd"
	case n%10 == 3:
		suffix = "rd"
	}
	return strconv.Itoa(n) + suffix
}
