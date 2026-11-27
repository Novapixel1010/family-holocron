package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/novapixel1010/family-holocron/internal/profile"
	"github.com/novapixel1010/family-holocron/plugin"
	"github.com/novapixel1010/family-holocron/plugin/builtin/stats"
	"github.com/novapixel1010/family-holocron/server"
	"github.com/novapixel1010/family-holocron/store"
	"github.com/novapixel1010/family-holocron/store/db/sqlite"
)

type env struct {
	t    *testing.T
	ts   *httptest.Server
	prof *profile.Profile
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	p, err := profile.Load([]string{"--data", filepath.Join(dir, "data"), "--media", filepath.Join(dir, "media")})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	db, err := sqlite.Open(ctx, p.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	reg := plugin.NewRegistry()
	reg.Register(stats.New())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := server.New(ctx, server.Options{Profile: p, Store: store.New(db), Plugins: reg, Logger: logger,
		SchemaVersion: sqlite.LatestSchemaVersion, Prepare: sqlite.PrepareRestoreSource})
	if err != nil {
		t.Fatal(err)
	}
	srv.Auth.BcryptCost = bcrypt.MinCost
	ts := httptest.NewServer(srv.Handler)
	t.Cleanup(ts.Close)
	return &env{t: t, ts: ts, prof: p}
}

type client struct {
	e *env
	c *http.Client
}

func (e *env) client() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, c: &http.Client{Jar: jar}}
}

func (c *client) do(method, path string, body any, out any) int {
	c.e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.e.ts.URL+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.c.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			c.e.t.Fatalf("%s %s: decode %q: %v", method, path, data, err)
		}
	}
	return resp.StatusCode
}

func (c *client) must(method, path string, body any, out any, want int) {
	c.e.t.Helper()
	var raw json.RawMessage
	got := c.do(method, path, body, &raw)
	if got != want {
		c.e.t.Fatalf("%s %s: status %d, want %d: %s", method, path, got, want, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			c.e.t.Fatal(err)
		}
	}
}

func (c *client) upload(path, field, filename string, content []byte, fields map[string]string, out any) int {
	c.e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	fw, _ := mw.CreateFormFile(field, filename)
	fw.Write(content)
	mw.Close()
	req, _ := http.NewRequest("POST", c.e.ts.URL+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.c.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if out != nil {
		json.Unmarshal(data, out)
	}
	if resp.StatusCode >= 300 {
		c.e.t.Logf("upload %s: %d %s", path, resp.StatusCode, data)
	}
	return resp.StatusCode
}

func (c *client) raw(path string) (int, []byte, http.Header) {
	resp, err := c.c.Get(c.e.ts.URL + path)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b, resp.Header
}

type person struct {
	ID        int64  `json:"id"`
	GivenName string `json:"givenName"`
	Surname   string `json:"surname"`
	About     string `json:"about"`
	CreatedBy *struct {
		Username string `json:"username"`
	} `json:"createdBy"`
	UpdatedBy *struct {
		Username string `json:"username"`
	} `json:"updatedBy"`
}

// A 1x1 PNG.
var pngBytes = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0x0d, 0x49, 0x48, 0x44, 0x52, 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1f, 0x15, 0xc4, 0x89, 0, 0, 0, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0, 1, 0, 0, 5, 0, 1, 0x0d, 0x0a, 0x2d, 0xb4, 0, 0, 0, 0, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82}

func setupAdmin(t *testing.T, e *env) *client {
	admin := e.client()
	var status map[string]any
	admin.must("GET", "/api/v1/status", nil, &status, 200)
	if status["setupRequired"] != true {
		t.Fatal("fresh instance should require setup")
	}
	admin.must("POST", "/api/v1/auth/setup", map[string]string{"username": "root", "password": "correct-horse", "displayName": "Root Admin"}, nil, 201)
	return admin
}

func TestSetupAuthAndAttribution(t *testing.T) {
	e := newEnv(t)
	admin := setupAdmin(t, e)
	// Setup only once.
	other := e.client()
	other.must("POST", "/api/v1/auth/setup", map[string]string{"username": "evil", "password": "password123"}, nil, 409)
	// Unauthenticated access is rejected.
	other.must("GET", "/api/v1/persons", nil, nil, 401)
	other.must("POST", "/api/v1/auth/login", map[string]string{"username": "root", "password": "wrong-password"}, nil, 401)

	admin.must("POST", "/api/v1/users", map[string]string{"username": "eddie", "password": "editor-pass", "role": "editor", "displayName": "Eddie"}, nil, 201)
	admin.must("POST", "/api/v1/users", map[string]string{"username": "vera", "password": "viewer-pass", "role": "viewer"}, nil, 201)

	var p person
	admin.must("POST", "/api/v1/persons", map[string]any{"givenName": "Margaret", "surname": "Holloway", "birthDate": "1921-04-02",
		"about": "## Early life\nGrew up on the farm."}, &p, 201)
	if p.CreatedBy == nil || p.CreatedBy.Username != "root" {
		t.Fatalf("createdBy: %+v", p.CreatedBy)
	}

	editor := e.client()
	editor.must("POST", "/api/v1/auth/login", map[string]string{"username": "eddie", "password": "editor-pass"}, nil, 200)
	var upd person
	editor.must("PUT", fmt.Sprintf("/api/v1/persons/%d", p.ID), map[string]any{"givenName": "Margaret", "surname": "Holloway-Reed",
		"birthDate": "1921-04-02", "about": p.About}, &upd, 200)
	if upd.CreatedBy.Username != "root" || upd.UpdatedBy.Username != "eddie" {
		t.Fatalf("attribution after edit: created=%v updated=%v", upd.CreatedBy, upd.UpdatedBy)
	}

	viewer := e.client()
	viewer.must("POST", "/api/v1/auth/login", map[string]string{"username": "vera", "password": "viewer-pass"}, nil, 200)
	viewer.must("GET", fmt.Sprintf("/api/v1/persons/%d", p.ID), nil, nil, 200)
	viewer.must("POST", "/api/v1/persons", map[string]any{"givenName": "Nope"}, nil, 403)
	viewer.must("GET", "/api/v1/backups", nil, nil, 403)

	// Audit log records both users.
	var audit struct {
		Items []struct {
			Action   string                    `json:"action"`
			Username string                    `json:"username"`
			Changes  map[string]map[string]any `json:"changes"`
		} `json:"items"`
	}
	viewer.must("GET", fmt.Sprintf("/api/v1/audit?personId=%d", p.ID), nil, &audit, 200)
	if len(audit.Items) != 2 || audit.Items[0].Username != "eddie" || audit.Items[1].Username != "root" {
		t.Fatalf("audit: %+v", audit.Items)
	}
	if audit.Items[0].Changes["surname"]["to"] != "Holloway-Reed" {
		t.Fatalf("diff: %+v", audit.Items[0].Changes)
	}
	// Validation errors carry field info.
	var verr struct {
		Error struct {
			Code   string            `json:"code"`
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	editor.do("POST", "/api/v1/persons", map[string]any{"sex": "Q"}, &verr)
	if verr.Error.Code != "validation" || verr.Error.Fields["sex"] == "" || verr.Error.Fields["givenName"] == "" {
		t.Fatalf("validation error: %+v", verr)
	}
	// Cross-origin writes are rejected.
	req, _ := http.NewRequest("POST", e.ts.URL+"/api/v1/persons", strings.NewReader(`{"givenName":"x"}`))
	req.Header.Set("Origin", "https://evil.example")
	resp, _ := editor.c.Do(req)
	if resp.StatusCode != 403 {
		t.Fatalf("cross-origin write: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestTreeTimelineAndFamily(t *testing.T) {
	e := newEnv(t)
	admin := setupAdmin(t, e)
	mk := func(given, sex, birth string) int64 {
		var p person
		admin.must("POST", "/api/v1/persons", map[string]any{"givenName": given, "surname": "Tree", "sex": sex, "birthDate": birth}, &p, 201)
		return p.ID
	}
	kid, dad, mom, gpa := mk("Kid", "M", "1990"), mk("Dad", "M", "1960"), mk("Mom", "F", "1962"), mk("Grandpa", "M", "1930")
	rel := func(t, a, b int64) {
		admin.must("POST", "/api/v1/relationships", map[string]any{"type": "parent", "person1Id": a, "person2Id": b}, nil, 201)
	}
	rel(0, mom, kid)
	rel(0, dad, kid)
	rel(0, gpa, dad)
	admin.must("POST", "/api/v1/relationships", map[string]any{"type": "spouse", "person1Id": dad, "person2Id": mom}, nil, 201)
	admin.must("POST", "/api/v1/relationships", map[string]any{"type": "parent", "person1Id": kid, "person2Id": gpa}, nil, 422)

	var tree struct {
		Root struct {
			Person   struct{ Name string } `json:"person"`
			Children []struct {
				Person   struct{ Name string } `json:"person"`
				HasMore  bool                  `json:"hasMore"`
				Children []any                 `json:"children"`
			} `json:"children"`
		} `json:"root"`
		Count int `json:"count"`
	}
	admin.must("GET", fmt.Sprintf("/api/v1/tree/%d?generations=1", kid), nil, &tree, 200)
	if tree.Count != 3 || len(tree.Root.Children) != 2 || tree.Root.Children[0].Person.Name != "Dad Tree" {
		t.Fatalf("tree: %+v", tree)
	}
	if !tree.Root.Children[0].HasMore || tree.Root.Children[1].HasMore {
		t.Fatalf("hasMore flags: %+v", tree.Root.Children)
	}
	admin.must("GET", fmt.Sprintf("/api/v1/tree/%d?generations=3", kid), nil, &tree, 200)
	if tree.Count != 4 {
		t.Fatalf("deep tree count %d", tree.Count)
	}

	var fam struct {
		Parents, Spouses, Children, Siblings []any
	}
	admin.must("GET", fmt.Sprintf("/api/v1/persons/%d/family", dad), nil, &fam, 200)
	if len(fam.Parents) != 1 || len(fam.Spouses) != 1 || len(fam.Children) != 1 {
		t.Fatalf("family: %+v", fam)
	}

	admin.must("POST", "/api/v1/events", map[string]any{"type": "marriage", "date": "1985-06-01", "place": "Leeds",
		"participants": []map[string]any{{"personId": dad}, {"personId": mom}}}, nil, 201)
	admin.must("POST", "/api/v1/events", map[string]any{"type": "historical", "title": "Moon landing", "date": "1969-07-20"}, nil, 201)
	var tl struct {
		Items []struct {
			Kind, Type, Title, SortDate string
		} `json:"items"`
	}
	admin.must("GET", "/api/v1/timeline", nil, &tl, 200)
	var order []string
	for _, it := range tl.Items {
		order = append(order, it.Type)
	}
	want := "birth,birth,birth,historical,marriage,birth"
	if strings.Join(order, ",") != want {
		t.Fatalf("timeline order %v, want %s", order, want)
	}
	admin.must("GET", fmt.Sprintf("/api/v1/timeline?personId=%d&types=marriage", dad), nil, &tl, 200)
	if len(tl.Items) != 1 || tl.Items[0].Type != "marriage" {
		t.Fatalf("filtered timeline: %+v", tl.Items)
	}
	admin.must("GET", "/api/v1/timeline?from=1960&to=1969", nil, &tl, 200)
	if len(tl.Items) != 3 {
		t.Fatalf("range timeline: %+v", tl.Items)
	}
}

func TestBackupAndRestoreRoundTrip(t *testing.T) {
	e := newEnv(t)
	admin := setupAdmin(t, e)
	var keep person
	admin.must("POST", "/api/v1/persons", map[string]any{"givenName": "Keeper", "about": "Story to preserve"}, &keep, 201)
	var media struct {
		ID int64 `json:"id"`
	}
	if code := admin.upload("/api/v1/media", "file", "portrait.png", pngBytes, map[string]string{"personIds": fmt.Sprint(keep.ID)}, &media); code != 201 {
		t.Fatalf("media upload: %d", code)
	}
	code, body, hdr := admin.raw(fmt.Sprintf("/api/v1/media/%d/file", media.ID))
	if code != 200 || !bytes.Equal(body, pngBytes) || hdr.Get("Content-Type") != "image/png" {
		t.Fatalf("media file: %d %s", code, hdr.Get("Content-Type"))
	}
	if code := admin.upload("/api/v1/media", "file", "evil.html", []byte("<html><script>alert(1)</script></html>"), nil, nil); code != 422 {
		t.Fatalf("html upload should be rejected, got %d", code)
	}

	// Download a fresh backup.
	code, archive, hdr := admin.raw("/api/v1/backups/download")
	if code != 200 || len(archive) == 0 || !strings.Contains(hdr.Get("Content-Disposition"), "holocron-backup-") {
		t.Fatalf("download backup: %d", code)
	}

	// Diverge: delete the person and media, add someone new.
	admin.must("DELETE", fmt.Sprintf("/api/v1/persons/%d", keep.ID), nil, nil, 204)
	admin.must("DELETE", fmt.Sprintf("/api/v1/media/%d", media.ID), nil, nil, 204)
	admin.must("POST", "/api/v1/persons", map[string]any{"givenName": "Interloper"}, nil, 201)

	// Corrupt archives are rejected before anything changes.
	if code := admin.upload("/api/v1/backups/upload", "file", "bad.tar.gz", []byte("not a backup"), nil, nil); code != 422 {
		t.Fatalf("bad archive upload: %d", code)
	}
	tampered := append([]byte{}, archive...)
	tampered[len(tampered)/2] ^= 0xff
	if code := admin.upload("/api/v1/backups/upload", "file", "tampered.tar.gz", tampered, nil, nil); code != 422 {
		t.Fatalf("tampered archive upload: %d", code)
	}

	var info struct {
		Name     string `json:"name"`
		Kind     string `json:"kind"`
		Manifest struct {
			Counts struct {
				Persons int `json:"persons"`
				Media   int `json:"media"`
			} `json:"counts"`
		} `json:"manifest"`
	}
	if code := admin.upload("/api/v1/backups/upload", "file", "mine.tar.gz", archive, nil, &info); code != 201 {
		t.Fatalf("upload backup: %d", code)
	}
	if info.Kind != "upload" || info.Manifest.Counts.Persons != 1 || info.Manifest.Counts.Media != 1 {
		t.Fatalf("uploaded info: %+v", info)
	}
	// Explicit confirmation is required.
	admin.must("POST", "/api/v1/backups/"+info.Name+"/restore", map[string]string{"confirm": "yes"}, nil, 422)

	var res struct {
		Result struct {
			SafetySnapshot string `json:"safetySnapshot"`
			SessionKept    bool   `json:"sessionKept"`
		} `json:"result"`
	}
	admin.must("POST", "/api/v1/backups/"+info.Name+"/restore", map[string]string{"confirm": "RESTORE"}, &res, 200)
	if !res.Result.SessionKept || !strings.HasPrefix(res.Result.SafetySnapshot, "pre-restore-") {
		t.Fatalf("restore result: %+v", res)
	}

	// Data, attribution and media are back; the interloper is gone.
	var list struct {
		Items []person `json:"items"`
	}
	admin.must("GET", "/api/v1/persons", nil, &list, 200)
	if len(list.Items) != 1 || list.Items[0].GivenName != "Keeper" || list.Items[0].About != "Story to preserve" || list.Items[0].CreatedBy.Username != "root" {
		t.Fatalf("restored persons: %+v", list.Items)
	}
	code, body, _ = admin.raw(fmt.Sprintf("/api/v1/media/%d/file", media.ID))
	if code != 200 || !bytes.Equal(body, pngBytes) {
		t.Fatalf("restored media file: %d", code)
	}

	// The safety snapshot is listed and itself restorable.
	var backups []struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
	}
	admin.must("GET", "/api/v1/backups", nil, &backups, 200)
	kinds := map[string]int{}
	for _, b := range backups {
		kinds[b.Kind]++
	}
	if kinds["safety"] != 1 || kinds["upload"] != 1 || kinds["manual"] != 1 {
		t.Fatalf("backup list kinds: %+v", kinds)
	}
	admin.must("POST", "/api/v1/backups/"+res.Result.SafetySnapshot+"/restore", map[string]string{"confirm": "RESTORE"}, nil, 200)
	admin.must("GET", "/api/v1/persons", nil, &list, 200)
	if len(list.Items) != 1 || list.Items[0].GivenName != "Interloper" {
		t.Fatalf("after rolling back to safety snapshot: %+v", list.Items)
	}
	// The restore itself is in the audit log.
	var audit struct {
		Items []struct{ Action string } `json:"items"`
	}
	admin.must("GET", "/api/v1/audit?entityType=system", nil, &audit, 200)
	if len(audit.Items) == 0 || audit.Items[0].Action != "restore" {
		t.Fatalf("audit after restore: %+v", audit.Items)
	}
	// Leftover staging directories are cleaned up.
	entries, _ := os.ReadDir(e.prof.BackupDir)
	for _, en := range entries {
		if strings.HasPrefix(en.Name(), ".") {
			t.Fatalf("leftover temp entry %s", en.Name())
		}
	}
	// Path traversal in backup names is refused.
	admin.must("GET", "/api/v1/backups/..%2f..%2fetc%2fpasswd/download", nil, nil, 404)
}

func TestGEDCOMRoundTrip(t *testing.T) {
	e := newEnv(t)
	admin := setupAdmin(t, e)
	ged := `0 HEAD
1 GEDC
2 VERS 5.5.1
1 CHAR UTF-8
0 @I1@ INDI
1 NAME John /Smith/
1 SEX M
1 BIRT
2 DATE 12 MAR 1850
2 PLAC York, England
1 DEAT
2 DATE ABT 1910
1 OCCU Blacksmith
1 NOTE Known for his
2 CONT generosity.
1 FAMS @F1@
0 @I2@ INDI
1 NAME Mary /Jones/
1 SEX F
1 FAMS @F1@
0 @I3@ INDI
1 NAME William /Smith/
1 SEX M
1 BIRT
2 DATE 1875
1 FAMC @F1@
0 @F1@ FAM
1 HUSB @I1@
1 WIFE @I2@
1 CHIL @I3@
1 MARR
2 DATE 1874
2 PLAC Leeds
0 TRLR
`
	var res struct {
		Persons, Relationships, Events int
	}
	if code := admin.upload("/api/v1/gedcom/import", "file", "smith.ged", []byte(ged), nil, &res); code != 200 {
		t.Fatalf("import: %d", code)
	}
	if res.Persons != 3 || res.Relationships != 3 || res.Events != 2 {
		t.Fatalf("import result %+v", res)
	}
	var list struct {
		Items []struct {
			GivenName, Surname, BirthDate, DeathDate, About string
		} `json:"items"`
	}
	admin.must("GET", "/api/v1/persons?q=john", nil, &list, 200)
	if len(list.Items) != 1 || list.Items[0].BirthDate != "1850-03-12" || list.Items[0].DeathDate != "ABT 1910" ||
		list.Items[0].About != "Known for his\ngenerosity." {
		t.Fatalf("imported john: %+v", list.Items)
	}
	code, out, _ := admin.raw("/api/v1/gedcom/export")
	if code != 200 {
		t.Fatalf("export: %d", code)
	}
	s := string(out)
	for _, want := range []string{"1 NAME John /Smith/", "2 DATE 12 MAR 1850", "1 HUSB @I", "1 CHIL @I", "1 MARR", "2 PLAC Leeds", "1 OCCU Blacksmith", "2 CONT generosity."} {
		if !strings.Contains(s, want) {
			t.Fatalf("export missing %q:\n%s", want, s)
		}
	}
	// Re-import the export into a fresh instance: same shape.
	e2 := newEnv(t)
	admin2 := setupAdmin(t, e2)
	var res2 struct{ Persons, Relationships, Events int }
	if code := admin2.upload("/api/v1/gedcom/import", "file", "roundtrip.ged", out, nil, &res2); code != 200 {
		t.Fatalf("re-import: %d", code)
	}
	if res2 != res {
		t.Fatalf("round trip mismatch: %+v vs %+v", res2, res)
	}
	admin.must("POST", "/api/v1/gedcom/import", nil, nil, 400)
}

func TestPluginsAndFrontendFallback(t *testing.T) {
	e := newEnv(t)
	admin := setupAdmin(t, e)
	var plugins []struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
		Slots   []struct{ Kind string }
	}
	admin.must("GET", "/api/v1/plugins", nil, &plugins, 200)
	if len(plugins) != 1 || plugins[0].ID != "stats" || !plugins[0].Enabled || plugins[0].Slots[0].Kind != "nav" {
		t.Fatalf("plugins: %+v", plugins)
	}
	admin.must("POST", "/api/v1/persons", map[string]any{"givenName": "A", "surname": "Zed", "birthDate": "1801", "deathDate": "1871"}, nil, 201)
	time.Sleep(10 * time.Millisecond)
	var sum struct {
		People          int     `json:"people"`
		AverageLifespan float64 `json:"averageLifespan"`
		ChangesRecorded int     `json:"changesRecorded"`
	}
	admin.must("GET", "/api/v1/plugins/stats/summary", nil, &sum, 200)
	if sum.People != 1 || sum.AverageLifespan != 70 || sum.ChangesRecorded < 1 {
		t.Fatalf("stats: %+v", sum)
	}
	admin.must("PUT", "/api/v1/plugins/stats", map[string]bool{"enabled": false}, nil, 200)
	admin.must("GET", "/api/v1/plugins/stats/summary", nil, nil, 404)
	admin.must("PUT", "/api/v1/plugins/nope", map[string]bool{"enabled": true}, nil, 404)

	admin.must("GET", "/api/v1/does-not-exist", nil, nil, 404)
	code, body, _ := admin.raw("/people/42")
	if code != 200 && code != 503 {
		t.Fatalf("SPA fallback status %d", code)
	}
	if !strings.Contains(strings.ToLower(string(body)), "<!doctype html>") {
		t.Fatalf("SPA fallback should return HTML: %s", body)
	}
	code, _, _ = admin.raw("/healthz")
	if code != 200 {
		t.Fatalf("healthz: %d", code)
	}
}

// TestTreeGraphIncludesEveryRelative covers the flat graph used by the tree view:
// shared ancestors appear once, couples are linked, and co-parents/spouses are
// included in descendant views even without a spouse record.
func TestTreeGraphIncludesEveryRelative(t *testing.T) {
	e := newEnv(t)
	admin := setupAdmin(t, e)
	mk := func(given, sex string) int64 {
		var p person
		admin.must("POST", "/api/v1/persons", map[string]any{"givenName": given, "surname": "Graph", "sex": sex, "about": map[bool]string{true: "story"}[given == "Kid"]}, &p, 201)
		return p.ID
	}
	link := func(typ string, a, b int64) {
		admin.must("POST", "/api/v1/relationships", map[string]any{"type": typ, "person1Id": a, "person2Id": b}, nil, 201)
	}
	// Two cousins (Dad, Mom) who share grandparents GP1+GP2 → pedigree collapse for Kid.
	gp1, gp2 := mk("GP1", "M"), mk("GP2", "F")
	auntA, uncleB := mk("AuntA", "F"), mk("UncleB", "M")
	dad, mom, kid := mk("Dad", "M"), mk("Mom", "F"), mk("Kid", "U")
	link("parent", gp1, auntA)
	link("parent", gp2, auntA)
	link("parent", gp1, uncleB)
	link("parent", gp2, uncleB)
	link("parent", auntA, dad)
	link("parent", uncleB, mom)
	link("parent", dad, kid)
	link("parent", mom, kid)
	link("spouse", dad, mom)
	link("spouse", gp1, gp2)
	// Kid has a child with a partner who has no spouse record, and a childless spouse.
	partner, spouse2, grandkid := mk("Partner", "F"), mk("Spouse2", "M"), mk("Grandkid", "U")
	link("parent", kid, grandkid)
	link("parent", partner, grandkid)
	link("spouse", kid, spouse2)

	type graph struct {
		Count  int `json:"count"`
		People []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			Role       string `json:"role"`
			Generation int    `json:"generation"`
			HasMore    bool   `json:"hasMore"`
			HasAbout   bool   `json:"hasAbout"`
		} `json:"people"`
		Links []struct {
			ID             int64  `json:"id"`
			Type           string `json:"type"`
			Source, Target int64
		} `json:"links"`
	}
	var g graph
	admin.must("GET", fmt.Sprintf("/api/v1/tree/%d?generations=4", kid), nil, &g, 200)
	seen := map[int64]int{}
	for _, p := range g.People {
		seen[p.ID]++
	}
	for _, id := range []int64{kid, dad, mom, auntA, uncleB, gp1, gp2} {
		if seen[id] != 1 {
			t.Fatalf("ancestor %d appears %d times in graph: %+v", id, seen[id], g.People)
		}
	}
	if len(g.People) != 7 || g.Count != 9 {
		t.Fatalf("want 7 unique people (nested count 9 with duplicates), got %d / %d", len(g.People), g.Count)
	}
	// 8 parent links among them + Dad&Mom + GP1&GP2 couples.
	spouseLinks, parentLinks := 0, 0
	ids := map[int64]bool{}
	for _, l := range g.Links {
		if ids[l.ID] {
			t.Fatalf("duplicate link id %d", l.ID)
		}
		ids[l.ID] = true
		if l.Type == "spouse" {
			spouseLinks++
		} else {
			parentLinks++
		}
	}
	if parentLinks != 8 || spouseLinks != 2 {
		t.Fatalf("links: %d parent, %d spouse: %+v", parentLinks, spouseLinks, g.Links)
	}
	for _, p := range g.People {
		if p.ID == kid && (!p.HasAbout || p.Role != "root") {
			t.Fatalf("root flags: %+v", p)
		}
	}

	// Depth limit: grandparents sit beyond generation 1, so their children are flagged.
	admin.must("GET", fmt.Sprintf("/api/v1/tree/%d?generations=1", kid), nil, &g, 200)
	if len(g.People) != 3 {
		t.Fatalf("1 generation: %+v", g.People)
	}
	for _, p := range g.People {
		if (p.ID == dad || p.ID == mom) != p.HasMore {
			t.Fatalf("hasMore for %s = %v", p.Name, p.HasMore)
		}
	}

	// Descendants of Dad: Kid, Grandkid, plus partners Mom (co-parent), Partner (co-parent, no spouse
	// record) and Spouse2 (childless spouse).
	admin.must("GET", fmt.Sprintf("/api/v1/tree/%d?direction=descendants&generations=3", dad), nil, &g, 200)
	roles := map[int64]string{}
	gens := map[int64]int{}
	for _, p := range g.People {
		roles[p.ID] = p.Role
		gens[p.ID] = p.Generation
	}
	want := map[int64]string{dad: "root", kid: "descendant", grandkid: "descendant", mom: "partner", partner: "partner", spouse2: "partner"}
	if len(roles) != len(want) {
		t.Fatalf("descendant graph people: %+v", g.People)
	}
	for id, r := range want {
		if roles[id] != r {
			t.Fatalf("role of %d = %q, want %q (%+v)", id, roles[id], r, g.People)
		}
	}
	if gens[partner] != 1 || gens[spouse2] != 1 || gens[mom] != 0 || gens[grandkid] != 2 {
		t.Fatalf("partner generations: %+v", gens)
	}
	hasLink := func(typ string, a, b int64) bool {
		for _, l := range g.Links {
			if l.Type == typ && l.Source == a && l.Target == b {
				return true
			}
		}
		return false
	}
	if !hasLink("parent", partner, grandkid) || !hasLink("parent", mom, kid) || !hasLink("spouse", min(kid, spouse2), max(kid, spouse2)) {
		t.Fatalf("descendant links missing: %+v", g.Links)
	}
}
