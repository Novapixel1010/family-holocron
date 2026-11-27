// Package exchange implements portable import and export (GEDCOM 5.5.1 and JSON).
package exchange

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/novapixel1010/family-holocron/internal/gedcom"
	"github.com/novapixel1010/family-holocron/internal/gendate"
	"github.com/novapixel1010/family-holocron/internal/version"
	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/server/module/backup"
	"github.com/novapixel1010/family-holocron/store"
)

// Module is the import/export module.
type Module struct {
	Store          *store.Store
	MaxUploadBytes int64
}

// Register mounts routes.
func (m *Module) Register(rt *apiutil.Router) {
	rt.Handle("POST /api/v1/gedcom/import", apiutil.Editor, m.importGEDCOM)
	rt.Handle("GET /api/v1/gedcom/export", apiutil.Viewer, m.exportGEDCOM)
	rt.Handle("GET /api/v1/export/json", apiutil.Viewer, m.exportJSON)
}

// tagToType maps GEDCOM individual/family event tags to Holocron event types.
var tagToType = map[string]string{
	"RESI": "residence", "BAPM": "baptism", "CHR": "christening", "BURI": "burial", "CREM": "cremation",
	"EDUC": "education", "GRAD": "graduation", "OCCU": "occupation", "EMIG": "emigration", "IMMI": "immigration",
	"NATU": "naturalization", "RELI": "religion", "RETI": "retirement", "CENS": "census", "EVEN": "custom",
	"MARR": "marriage", "DIV": "divorce", "ENGA": "engagement", "_MILT": "military", "_MDCL": "medical",
}

var typeToTag = func() map[string]string {
	out := map[string]string{}
	for k, v := range tagToType {
		out[v] = k
	}
	out["custom"] = "EVEN"
	out["family"] = "EVEN"
	out["historical"] = "EVEN"
	return out
}()

var nameRe = regexp.MustCompile(`^(.*?)/(.*?)/(.*)$`)

func parseName(n *gedcom.Node) (given, surname string) {
	if n == nil {
		return "", ""
	}
	given, surname = strings.TrimSpace(n.Val("GIVN")), strings.TrimSpace(n.Val("SURN"))
	if given != "" || surname != "" {
		return
	}
	if mm := nameRe.FindStringSubmatch(n.Value); mm != nil {
		given = strings.TrimSpace(mm[1] + " " + strings.TrimSpace(mm[3]))
		return strings.TrimSpace(given), strings.TrimSpace(mm[2])
	}
	return strings.TrimSpace(n.Value), ""
}

// ParseGEDCOM converts a GEDCOM stream to an import batch.
func ParseGEDCOM(r io.Reader, source string) (*store.ImportBatch, error) {
	roots, err := gedcom.Parse(r)
	if err != nil {
		return nil, err
	}
	b := &store.ImportBatch{Source: source}
	notes := map[string]string{}
	for _, rec := range roots {
		if rec.Tag == "NOTE" && rec.Xref != "" {
			notes[rec.Xref] = rec.Text()
		}
	}
	noteText := func(n *gedcom.Node) string {
		if strings.HasPrefix(n.Value, "@") && strings.HasSuffix(n.Value, "@") {
			return notes[n.Value]
		}
		return n.Text()
	}
	// pedigree[child][fam] = PEDI value
	pedigree := map[string]map[string]string{}
	for _, rec := range roots {
		if rec.Tag != "INDI" || rec.Xref == "" {
			continue
		}
		given, surname := parseName(rec.Child("NAME"))
		in := store.PersonInput{GivenName: given, Surname: surname, Sex: strings.ToUpper(strings.TrimSpace(rec.Val("SEX")))}
		if nameNode := rec.Child("NAME"); nameNode != nil {
			in.Nickname = nameNode.Val("NICK")
		}
		if in.Sex != "M" && in.Sex != "F" && in.Sex != "X" {
			in.Sex = "U"
		}
		if birt := rec.Child("BIRT"); birt != nil {
			in.BirthDate, in.BirthPlace = gendate.FromGEDCOM(birt.Val("DATE")), birt.Val("PLAC")
		}
		if deat := rec.Child("DEAT"); deat != nil {
			in.DeathDate, in.DeathPlace = gendate.FromGEDCOM(deat.Val("DATE")), deat.Val("PLAC")
			in.Deceased = true
		}
		var about []string
		for _, n := range rec.All("NOTE") {
			if t := strings.TrimSpace(noteText(n)); t != "" {
				about = append(about, t)
			}
		}
		in.About = strings.Join(about, "\n\n")
		if in.GivenName == "" && in.Surname == "" {
			in.GivenName = "Unknown"
		}
		b.Persons = append(b.Persons, store.ImportPerson{Ref: rec.Xref, Input: in})
		for _, c := range rec.Children {
			typ, ok := tagToType[c.Tag]
			if !ok || c.Tag == "MARR" || c.Tag == "DIV" || c.Tag == "ENGA" {
				continue
			}
			ev := store.EventInput{Type: typ, Date: gendate.FromGEDCOM(c.Val("DATE")), Place: c.Val("PLAC"), Description: c.Val("NOTE")}
			switch c.Tag {
			case "EVEN":
				ev.Title = firstNonEmpty(c.Val("TYPE"), c.Value, "Event")
			case "OCCU", "EDUC", "RELI":
				ev.Title = strings.TrimSpace(c.Value)
			}
			b.Events = append(b.Events, store.ImportEvent{Input: ev, ParticipantRefs: []string{rec.Xref}})
		}
		for _, fc := range rec.All("FAMC") {
			if pedi := strings.ToLower(fc.Val("PEDI")); pedi != "" {
				if pedigree[rec.Xref] == nil {
					pedigree[rec.Xref] = map[string]string{}
				}
				pedigree[rec.Xref][fc.Value] = pedi
			}
		}
	}
	for _, rec := range roots {
		if rec.Tag != "FAM" {
			continue
		}
		var parents []string
		for _, tag := range []string{"HUSB", "WIFE"} {
			for _, p := range rec.All(tag) {
				if p.Value != "" {
					parents = append(parents, p.Value)
				}
			}
		}
		div := rec.Child("DIV") != nil
		if len(parents) == 2 {
			sub := "married"
			if rec.Child("MARR") == nil {
				sub = "partner"
			}
			if div {
				sub = "divorced"
			}
			b.Relationships = append(b.Relationships, store.ImportRelationship{Type: store.RelSpouse, Person1Ref: parents[0], Person2Ref: parents[1],
				Subtype: sub, StartDate: dateOf(rec.Child("MARR")), EndDate: dateOf(rec.Child("DIV"))})
		}
		for _, tag := range []string{"MARR", "DIV", "ENGA"} {
			if ev := rec.Child(tag); ev != nil && len(parents) > 0 {
				b.Events = append(b.Events, store.ImportEvent{
					Input:           store.EventInput{Type: tagToType[tag], Date: gendate.FromGEDCOM(ev.Val("DATE")), Place: ev.Val("PLAC")},
					ParticipantRefs: parents,
				})
			}
		}
		for _, c := range rec.All("CHIL") {
			sub := "biological"
			switch pedigree[c.Value][rec.Xref] {
			case "adopted":
				sub = "adopted"
			case "foster":
				sub = "foster"
			case "step":
				sub = "step"
			}
			for _, p := range parents {
				b.Relationships = append(b.Relationships, store.ImportRelationship{Type: store.RelParent, Person1Ref: p, Person2Ref: c.Value, Subtype: sub})
			}
		}
	}
	return b, nil
}

func dateOf(n *gedcom.Node) string {
	if n == nil {
		return ""
	}
	return gendate.FromGEDCOM(n.Val("DATE"))
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

func (m *Module) importGEDCOM(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, m.MaxUploadBytes)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		return apiutil.BadRequest("Expected a multipart upload with a GEDCOM file.")
	}
	defer r.MultipartForm.RemoveAll()
	f, hdr, err := r.FormFile("file")
	if err != nil {
		return store.Invalid("file", "choose a .ged file to import")
	}
	defer f.Close()
	batch, err := ParseGEDCOM(f, filepath.Base(hdr.Filename))
	if err != nil {
		return &store.ValidationError{Message: "Could not read GEDCOM file: " + err.Error(), Fields: map[string]string{"file": err.Error()}}
	}
	res, err := m.Store.Import(r.Context(), apiutil.ActorFrom(r.Context()), batch)
	if err != nil {
		return err
	}
	return apiutil.OK(w, res)
}

// WriteGEDCOM writes all records as GEDCOM 5.5.1.
func WriteGEDCOM(ctx context.Context, s *store.Store, out io.Writer) error {
	people, _, err := s.ListPersons(ctx, store.FindPerson{})
	if err != nil {
		return err
	}
	rels, err := s.ListRelationships(ctx, store.FindRelationship{})
	if err != nil {
		return err
	}
	events, err := s.ListEvents(ctx, store.FindEvent{})
	if err != nil {
		return err
	}
	byID := map[int64]*store.Person{}
	for _, p := range people {
		byID[p.ID] = p
	}
	// Build families keyed by sorted parent pair.
	type family struct {
		xref     string
		parents  []int64
		children []int64
		spouse   *store.Relationship
		events   []*store.Event
	}
	fams := map[string]*family{}
	var order []string
	key := func(ids ...int64) string {
		ids = slices.Clone(ids)
		slices.Sort(ids)
		return fmt.Sprint(ids)
	}
	get := func(ids ...int64) *family {
		k := key(ids...)
		if f, ok := fams[k]; ok {
			return f
		}
		ids = slices.Clone(ids)
		slices.Sort(ids)
		f := &family{parents: ids}
		fams[k] = f
		order = append(order, k)
		return f
	}
	parentsOf := map[int64][]*store.Relationship{}
	for _, r := range rels {
		switch r.Type {
		case store.RelSpouse:
			get(r.Person1ID, r.Person2ID).spouse = r
		case store.RelParent:
			parentsOf[r.Person2ID] = append(parentsOf[r.Person2ID], r)
		}
	}
	childPedi := map[int64]map[string]string{} // child -> fam key -> pedi
	for child, prs := range parentsOf {
		var ids []int64
		for _, r := range prs {
			ids = append(ids, r.Person1ID)
		}
		groups := [][]int64{}
		if len(ids) <= 2 {
			groups = append(groups, ids)
		} else {
			groups = append(groups, ids[:2])
			for _, id := range ids[2:] {
				groups = append(groups, []int64{id})
			}
		}
		for _, g := range groups {
			f := get(g...)
			f.children = append(f.children, child)
			if prs[0].Subtype != "biological" && prs[0].Subtype != "unknown" {
				if childPedi[child] == nil {
					childPedi[child] = map[string]string{}
				}
				childPedi[child][key(g...)] = prs[0].Subtype
			}
		}
	}
	personEvents := map[int64][]*store.Event{}
	for _, e := range events {
		if len(e.Participants) == 0 {
			continue
		}
		if (e.Type == "marriage" || e.Type == "divorce" || e.Type == "engagement") && len(e.Participants) == 2 {
			if f, ok := fams[key(e.Participants[0].PersonID, e.Participants[1].PersonID)]; ok {
				f.events = append(f.events, e)
				continue
			}
		}
		personEvents[e.Participants[0].PersonID] = append(personEvents[e.Participants[0].PersonID], e)
	}
	for i, k := range order {
		fams[k].xref = fmt.Sprintf("@F%d@", i+1)
	}
	famsOf := map[int64][]string{}
	famcOf := map[int64][]string{}
	for _, k := range order {
		f := fams[k]
		for _, p := range f.parents {
			famsOf[p] = append(famsOf[p], k)
		}
		for _, c := range f.children {
			famcOf[c] = append(famcOf[c], k)
		}
	}

	g := gedcom.NewWriter(out)
	g.Line(0, "", "HEAD", "")
	g.Line(1, "", "SOUR", "FAMILY_HOLOCRON")
	g.Line(2, "", "NAME", "Family Holocron")
	g.Line(2, "", "VERS", version.Version)
	g.Line(1, "", "DATE", strings.ToUpper(time.Now().UTC().Format("2 Jan 2006")))
	g.Line(1, "", "GEDC", "")
	g.Line(2, "", "VERS", "5.5.1")
	g.Line(2, "", "FORM", "LINEAGE-LINKED")
	g.Line(1, "", "CHAR", "UTF-8")
	writeEvent := func(level int, tag string, e *store.Event) {
		val := ""
		switch e.Type {
		case "occupation", "education", "religion":
			val = e.Title
		}
		g.Line(level, "", tag, val)
		if tag == "EVEN" {
			g.Line(level+1, "", "TYPE", firstNonEmpty(e.Title, e.Type))
		}
		if d := gendate.Parse(e.Date).GEDCOM(); d != "" {
			g.Line(level+1, "", "DATE", d)
		}
		if e.Place != "" {
			g.Line(level+1, "", "PLAC", e.Place)
		}
		if e.Description != "" {
			g.Line(level+1, "", "NOTE", e.Description)
		}
	}
	for _, p := range people {
		g.Line(0, fmt.Sprintf("@I%d@", p.ID), "INDI", "")
		g.Line(1, "", "NAME", strings.TrimSpace(p.GivenName+" /"+p.Surname+"/"))
		if p.GivenName != "" {
			g.Line(2, "", "GIVN", p.GivenName)
		}
		if p.Surname != "" {
			g.Line(2, "", "SURN", p.Surname)
		}
		if p.Nickname != "" {
			g.Line(2, "", "NICK", p.Nickname)
		}
		g.Line(1, "", "SEX", p.Sex)
		if p.BirthDate != "" || p.BirthPlace != "" {
			g.Line(1, "", "BIRT", "")
			if d := gendate.Parse(p.BirthDate).GEDCOM(); d != "" {
				g.Line(2, "", "DATE", d)
			}
			if p.BirthPlace != "" {
				g.Line(2, "", "PLAC", p.BirthPlace)
			}
		}
		if p.Deceased || p.DeathDate != "" || p.DeathPlace != "" {
			if p.DeathDate == "" && p.DeathPlace == "" {
				g.Line(1, "", "DEAT", "Y")
			} else {
				g.Line(1, "", "DEAT", "")
				if d := gendate.Parse(p.DeathDate).GEDCOM(); d != "" {
					g.Line(2, "", "DATE", d)
				}
				if p.DeathPlace != "" {
					g.Line(2, "", "PLAC", p.DeathPlace)
				}
			}
		}
		for _, e := range personEvents[p.ID] {
			writeEvent(1, firstNonEmpty(typeToTag[e.Type], "EVEN"), e)
		}
		if strings.TrimSpace(p.About) != "" {
			g.Line(1, "", "NOTE", p.About)
		}
		notes, err := s.ListNotes(ctx, p.ID)
		if err != nil {
			return err
		}
		for _, n := range notes {
			text := n.Body
			if n.Title != "" {
				text = n.Title + "\n\n" + n.Body
			}
			g.Line(1, "", "NOTE", strings.TrimSpace(text))
		}
		for _, k := range famcOf[p.ID] {
			g.Line(1, "", "FAMC", fams[k].xref)
			if pedi := childPedi[p.ID][k]; pedi == "adopted" || pedi == "foster" {
				g.Line(2, "", "PEDI", pedi)
			}
		}
		for _, k := range famsOf[p.ID] {
			g.Line(1, "", "FAMS", fams[k].xref)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	for _, k := range order {
		f := fams[k]
		g.Line(0, f.xref, "FAM", "")
		roles := assignRoles(f.parents, byID)
		for i, pid := range f.parents {
			g.Line(1, "", roles[i], fmt.Sprintf("@I%d@", pid))
		}
		for _, c := range f.children {
			g.Line(1, "", "CHIL", fmt.Sprintf("@I%d@", c))
		}
		hasMarr := false
		for _, e := range f.events {
			if e.Type == "marriage" {
				hasMarr = true
			}
			writeEvent(1, typeToTag[e.Type], e)
		}
		if f.spouse != nil {
			sub := f.spouse.Subtype
			if !hasMarr && (sub == "married" || sub == "divorced" || sub == "widowed") {
				g.Line(1, "", "MARR", "")
				if d := gendate.Parse(f.spouse.StartDate).GEDCOM(); d != "" {
					g.Line(2, "", "DATE", d)
				}
			}
			if sub == "divorced" && !slices.ContainsFunc(f.events, func(e *store.Event) bool { return e.Type == "divorce" }) {
				g.Line(1, "", "DIV", "")
				if d := gendate.Parse(f.spouse.EndDate).GEDCOM(); d != "" {
					g.Line(2, "", "DATE", d)
				}
			}
		}
	}
	g.Line(0, "", "TRLR", "")
	return g.Flush()
}

// assignRoles maps parents to HUSB/WIFE by sex (GEDCOM 5.5.1 requires those tags).
func assignRoles(ids []int64, byID map[int64]*store.Person) []string {
	roles := make([]string, len(ids))
	if len(ids) == 1 {
		if p := byID[ids[0]]; p != nil && p.Sex == store.SexFemale {
			return []string{"WIFE"}
		}
		return []string{"HUSB"}
	}
	a, b := byID[ids[0]], byID[ids[1]]
	roles[0], roles[1] = "HUSB", "WIFE"
	if a != nil && b != nil && a.Sex == store.SexFemale && b.Sex != store.SexFemale {
		roles[0], roles[1] = "WIFE", "HUSB"
	}
	return roles
}

func (m *Module) exportGEDCOM(w http.ResponseWriter, r *http.Request) error {
	var buf bytes.Buffer
	if err := WriteGEDCOM(r.Context(), m.Store, &buf); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/x-gedcom; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="family-holocron-%s.ged"`, time.Now().UTC().Format("20060102")))
	_, err := w.Write(buf.Bytes())
	return err
}

func (m *Module) exportJSON(w http.ResponseWriter, r *http.Request) error {
	ex, err := backup.BuildExport(r.Context(), m.Store)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="family-holocron-%s.json"`, time.Now().UTC().Format("20060102")))
	return apiutil.OK(w, ex)
}
