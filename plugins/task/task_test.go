package task_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/novapixel1010/family-holocron/internal/profile"
	"github.com/novapixel1010/family-holocron/plugin"
	"github.com/novapixel1010/family-holocron/plugins/task"
	"github.com/novapixel1010/family-holocron/server"
	"github.com/novapixel1010/family-holocron/store"
	"github.com/novapixel1010/family-holocron/store/db/sqlite"
)

type env struct {
	t   *testing.T
	ts  *httptest.Server
	db  *sqlite.DB
	dir string
}

// start boots the full server on dir (reusing any existing database there).
func start(t *testing.T, dir string) *env {
	t.Helper()
	p, err := profile.Load([]string{"--data", filepath.Join(dir, "data"), "--media", filepath.Join(dir, "media")})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(context.Background(), p.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	reg := plugin.NewRegistry()
	reg.Register(task.New())
	srv, err := server.New(context.Background(), server.Options{Profile: p, Store: store.New(db), Plugins: reg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SchemaVersion: sqlite.LatestSchemaVersion, Prepare: sqlite.PrepareRestoreSource})
	if err != nil {
		t.Fatal(err)
	}
	srv.Auth.BcryptCost = bcrypt.MinCost
	e := &env{t: t, ts: httptest.NewServer(srv.Handler), db: db, dir: dir}
	t.Cleanup(e.stop)
	return e
}

func (e *env) stop() {
	if e.ts != nil {
		e.ts.Close()
		e.db.Close()
		e.ts = nil
	}
}

type client struct {
	e *env
	c *http.Client
}

func (e *env) login(user, pass string) *client {
	jar, _ := cookiejar.New(nil)
	c := &client{e: e, c: &http.Client{Jar: jar}}
	c.must("POST", "/api/v1/auth/login", map[string]string{"username": user, "password": pass}, nil, 200)
	return c
}

func (c *client) do(method, path string, body, out any) (int, []byte) {
	c.e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.e.ts.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.c.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if out != nil && len(data) > 0 {
		_ = json.Unmarshal(data, out)
	}
	return resp.StatusCode, data
}

func (c *client) must(method, path string, body, out any, want int) {
	c.e.t.Helper()
	if got, data := c.do(method, path, body, out); got != want {
		c.e.t.Fatalf("%s %s: %d (want %d): %s", method, path, got, want, data)
	}
}

func (c *client) upload(path string, content []byte, out any) {
	c.e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "census.png")
	fw.Write(content)
	mw.Close()
	resp, err := c.c.Post(c.e.ts.URL+path, mw.FormDataContentType(), &buf)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		c.e.t.Fatalf("upload: %d %s", resp.StatusCode, data)
	}
	json.Unmarshal(data, out)
}

var png = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0x0d, 0x49, 0x48, 0x44, 0x52, 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1f, 0x15, 0xc4, 0x89, 0, 0, 0, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0, 1, 0, 0, 5, 0, 1, 0x0d, 0x0a, 0x2d, 0xb4, 0, 0, 0, 0, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82}

type taskJSON struct {
	ID            int64    `json:"id"`
	Title         string   `json:"title"`
	DescriptionMD string   `json:"descriptionMd"`
	Status        string   `json:"status"`
	Priority      string   `json:"priority"`
	Tags          []string `json:"tags"`
	ClaimedBy     *struct {
		Username string `json:"username"`
	} `json:"claimedBy"`
	ClaimedAt *string `json:"claimedAt"`
	People    []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"people"`
	Media []struct {
		ID int64 `json:"id"`
	} `json:"media"`
	CommentCount int                        `json:"commentCount"`
	UpdatedAt    string                     `json:"updatedAt"`
	CreatedBy    *struct{ Username string } `json:"createdBy"`
	UpdatedBy    *struct{ Username string } `json:"updatedBy"`
}

type commentJSON struct {
	ID     int64  `json:"id"`
	BodyMD string `json:"bodyMd"`
	Edited bool   `json:"edited"`
	Author *struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
	} `json:"author"`
	CreatedAt string `json:"createdAt"`
}

type apiErr struct {
	Error struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields"`
	} `json:"error"`
}

// setup creates an admin, two editors and a viewer, plus one person.
func setup(t *testing.T, e *env) (admin, ann, bob, viewer *client, personID int64) {
	jar, _ := cookiejar.New(nil)
	admin = &client{e: e, c: &http.Client{Jar: jar}}
	admin.must("POST", "/api/v1/auth/setup", map[string]string{"username": "admin", "password": "password123"}, nil, 201)
	for _, u := range []struct{ name, role string }{{"ann", "editor"}, {"bob", "editor"}, {"vic", "viewer"}} {
		admin.must("POST", "/api/v1/users", map[string]string{"username": u.name, "password": "password123", "role": u.role, "displayName": u.name}, nil, 201)
	}
	var p struct{ ID int64 }
	admin.must("POST", "/api/v1/persons", map[string]any{"givenName": "Walter", "surname": "Hale", "birthDate": "1890"}, &p, 201)
	return admin, e.login("ann", "password123"), e.login("bob", "password123"), e.login("vic", "password123"), p.ID
}

const base = "/api/v1/plugins/task"

func TestTaskLifecycleLinksAndAttribution(t *testing.T) {
	e := start(t, t.TempDir())
	_, ann, bob, viewer, walter := setup(t, e)
	var media struct{ ID int64 }
	ann.upload("/api/v1/media", png, &media)

	var tk taskJSON
	ann.must("POST", base+"/tasks", map[string]any{
		"title": "Find Walter in the 1911 census", "descriptionMd": "Check **Leeds** first.", "priority": "high",
		"tags": []string{"census", " Census ", "uk"}, "personIds": []int64{walter}, "mediaIds": []int64{media.ID},
	}, &tk, 201)
	if tk.Status != "open" || tk.ClaimedBy != nil || len(tk.Tags) != 2 || len(tk.People) != 1 || tk.People[0].Name != "Walter Hale" || len(tk.Media) != 1 {
		t.Fatalf("created task: %+v", tk)
	}
	if tk.CreatedBy.Username != "ann" || tk.UpdatedBy.Username != "ann" {
		t.Fatal("creator attribution")
	}
	var bad apiErr
	if code, _ := ann.do("POST", base+"/tasks", map[string]any{"title": " ", "status": "nope"}, &bad); code != 422 || bad.Error.Fields["title"] == "" || bad.Error.Fields["status"] == "" {
		t.Fatalf("validation: %d %+v", code, bad)
	}

	// Edit by someone else keeps the creator.
	var upd taskJSON
	bob.must("PUT", fmt.Sprintf("%s/tasks/%d", base, tk.ID), map[string]any{"title": tk.Title, "descriptionMd": tk.DescriptionMD, "priority": "medium",
		"status": "blocked", "tags": []string{"census"}, "personIds": []int64{walter}, "mediaIds": []int64{media.ID}}, &upd, 200)
	if upd.CreatedBy.Username != "ann" || upd.UpdatedBy.Username != "bob" || upd.Status != "blocked" {
		t.Fatalf("update attribution: %+v", upd)
	}

	// Filters.
	var list []taskJSON
	viewer.must("GET", fmt.Sprintf("%s/tasks?personId=%d", base, walter), nil, &list, 200)
	if len(list) != 1 {
		t.Fatalf("person filter: %d", len(list))
	}
	viewer.must("GET", base+"/tasks?tag=CENSUS&status=blocked,open", nil, &list, 200)
	if len(list) != 1 {
		t.Fatalf("tag/status filter: %d", len(list))
	}
	viewer.must("GET", base+"/tasks?status=done", nil, &list, 200)
	if len(list) != 0 {
		t.Fatal("done filter should be empty")
	}
	var tags []struct {
		Tag   string
		Count int
	}
	viewer.must("GET", base+"/tags", nil, &tags, 200)
	if len(tags) != 1 || tags[0].Tag != "census" {
		t.Fatalf("tags: %+v", tags)
	}

	// Task changes appear in the linked person's history.
	var hist struct {
		Items []struct {
			EntityType, Action, Username string
		}
	}
	viewer.must("GET", fmt.Sprintf("/api/v1/audit?personId=%d", walter), nil, &hist, 200)
	var taskEntries int
	for _, h := range hist.Items {
		if h.EntityType == "task" {
			taskEntries++
		}
	}
	if taskEntries != 2 {
		t.Fatalf("person history should include task create+update: %+v", hist.Items)
	}

	// Viewers are read-only.
	viewer.must("POST", base+"/tasks", map[string]any{"title": "x"}, nil, 403)
	viewer.must("POST", fmt.Sprintf("%s/tasks/%d/claim", base, tk.ID), nil, nil, 403)

	// Deleting a person or media item removes the link, not the task.
	ann.must("DELETE", fmt.Sprintf("/api/v1/media/%d", media.ID), nil, nil, 204)
	ann.must("DELETE", fmt.Sprintf("/api/v1/persons/%d", walter), nil, nil, 204)
	ann.must("GET", fmt.Sprintf("%s/tasks/%d", base, tk.ID), nil, &upd, 200)
	if len(upd.People) != 0 || len(upd.Media) != 0 {
		t.Fatalf("links should cascade: %+v", upd)
	}
	ann.must("DELETE", fmt.Sprintf("%s/tasks/%d", base, tk.ID), nil, nil, 204)
	ann.must("GET", fmt.Sprintf("%s/tasks/%d", base, tk.ID), nil, nil, 404)
}

func TestClaimRules(t *testing.T) {
	e := start(t, t.TempDir())
	admin, ann, bob, _, _ := setup(t, e)
	var tk taskJSON
	ann.must("POST", base+"/tasks", map[string]any{"title": "Order death certificate"}, &tk, 201)
	path := fmt.Sprintf("%s/tasks/%d", base, tk.ID)

	// Claiming an open task claims it and moves it to in_progress.
	var c taskJSON
	ann.must("POST", path+"/claim", nil, &c, 200)
	if c.ClaimedBy == nil || c.ClaimedBy.Username != "ann" || c.ClaimedAt == nil || c.Status != "in_progress" {
		t.Fatalf("claim: %+v", c)
	}
	ann.must("POST", path+"/claim", nil, &c, 200) // idempotent for the claimer

	// Someone else needs to confirm a take-over.
	var conflict apiErr
	if code, _ := bob.do("POST", path+"/claim", map[string]bool{"takeOver": false}, &conflict); code != 409 ||
		conflict.Error.Code != "already_claimed" || conflict.Error.Fields["claimedBy"] != "ann" {
		t.Fatalf("claim conflict: %d %+v", code, conflict)
	}
	// Only the claimer (or an admin) can release it.
	bob.must("POST", path+"/unclaim", nil, nil, 403)
	bob.must("POST", path+"/claim", map[string]bool{"takeOver": true}, &c, 200)
	if c.ClaimedBy.Username != "bob" || c.Status != "in_progress" {
		t.Fatalf("take over: %+v", c)
	}
	// Unclaiming an in-progress task releases it back to open.
	bob.must("POST", path+"/unclaim", nil, &c, 200)
	if c.ClaimedBy != nil || c.ClaimedAt != nil || c.Status != "open" {
		t.Fatalf("unclaim: %+v", c)
	}
	// Admin can release someone else's claim; blocked tasks stay blocked.
	ann.must("POST", path+"/claim", nil, &c, 200)
	ann.must("PUT", path, map[string]any{"title": tk.Title, "status": "blocked"}, &c, 200)
	if c.ClaimedBy == nil || c.Status != "blocked" {
		t.Fatalf("blocked keeps claim: %+v", c)
	}
	admin.must("POST", path+"/unclaim", nil, &c, 200)
	if c.ClaimedBy != nil || c.Status != "blocked" {
		t.Fatalf("admin unclaim of blocked: %+v", c)
	}
	// Claiming a blocked task keeps it blocked.
	bob.must("POST", path+"/claim", nil, &c, 200)
	if c.Status != "blocked" || c.ClaimedBy.Username != "bob" {
		t.Fatalf("claim blocked: %+v", c)
	}
	// Setting status open via edit releases the claim; done tasks can't be claimed.
	bob.must("PUT", path, map[string]any{"title": tk.Title, "status": "open"}, &c, 200)
	if c.ClaimedBy != nil {
		t.Fatal("reopening should release the claim")
	}
	bob.must("PUT", path, map[string]any{"title": tk.Title, "status": "done"}, &c, 200)
	if code, _ := ann.do("POST", path+"/claim", nil, &conflict); code != 409 || conflict.Error.Code != "task_done" {
		t.Fatalf("claim done task: %d", code)
	}
	// Starting work via edit claims it for the editor.
	ann.must("PUT", path, map[string]any{"title": tk.Title, "status": "in_progress"}, &c, 200)
	if c.ClaimedBy == nil || c.ClaimedBy.Username != "ann" {
		t.Fatalf("in_progress via edit should claim: %+v", c)
	}
	var mine []taskJSON
	ann.must("GET", base+"/tasks?claimedBy=me", nil, &mine, 200)
	if len(mine) != 1 {
		t.Fatalf("claimedBy=me: %d", len(mine))
	}
}

func TestConcurrentClaimsHaveOneWinner(t *testing.T) {
	e := start(t, t.TempDir())
	_, ann, bob, _, _ := setup(t, e)
	for i := 0; i < 10; i++ {
		var tk taskJSON
		ann.must("POST", base+"/tasks", map[string]any{"title": fmt.Sprintf("race %d", i)}, &tk, 201)
		var wg sync.WaitGroup
		codes := make([]int, 2)
		for j, c := range []*client{ann, bob} {
			wg.Add(1)
			go func(j int, c *client) {
				defer wg.Done()
				codes[j], _ = c.do("POST", fmt.Sprintf("%s/tasks/%d/claim", base, tk.ID), nil, nil)
			}(j, c)
		}
		wg.Wait()
		if !((codes[0] == 200 && codes[1] == 409) || (codes[0] == 409 && codes[1] == 200)) {
			t.Fatalf("race %d: want exactly one winner, got %v", i, codes)
		}
	}
}

func TestToggleSurvivesRestartAndGuardsRoutes(t *testing.T) {
	dir := t.TempDir()
	e := start(t, dir)
	admin, ann, _, _, walter := setup(t, e)
	ann.must("POST", base+"/tasks", map[string]any{"title": "Kept while off", "personIds": []int64{walter}}, nil, 201)

	var plugins []struct {
		ID      string
		Enabled bool
	}
	ann.must("PUT", "/api/v1/plugins/task", map[string]bool{"enabled": false}, nil, 403) // admins only
	admin.must("PUT", "/api/v1/plugins/task", map[string]bool{"enabled": false}, &plugins, 200)
	var off apiErr
	if code, _ := ann.do("GET", base+"/tasks", nil, &off); code != 404 || off.Error.Code != "plugin_disabled" {
		t.Fatalf("disabled route: %d %+v", code, off)
	}
	// Core genealogy keeps working, including deleting a person the task links to.
	ann.must("GET", "/api/v1/persons", nil, nil, 200)
	ann.must("GET", fmt.Sprintf("/api/v1/tree/%d", walter), nil, nil, 200)

	// Restart on the same data directory: still off.
	e.stop()
	e2 := start(t, dir)
	admin2 := e2.login("admin", "password123")
	admin2.must("GET", "/api/v1/plugins", nil, &plugins, 200)
	if len(plugins) != 1 || plugins[0].Enabled {
		t.Fatalf("plugin state after restart: %+v", plugins)
	}
	admin2.must("GET", base+"/tasks", nil, nil, 404)
	// Turning it back on restores the data untouched.
	admin2.must("PUT", "/api/v1/plugins/task", map[string]bool{"enabled": true}, nil, 200)
	var list []taskJSON
	admin2.must("GET", base+"/tasks", nil, &list, 200)
	if len(list) != 1 || list[0].Title != "Kept while off" || len(list[0].People) != 1 {
		t.Fatalf("tasks after re-enable: %+v", list)
	}
}

func TestBackupAndRestoreIncludeTasks(t *testing.T) {
	e := start(t, t.TempDir())
	admin, ann, _, _, walter := setup(t, e)
	var media struct{ ID int64 }
	ann.upload("/api/v1/media", png, &media)
	var tk taskJSON
	ann.must("POST", base+"/tasks", map[string]any{"title": "Scan the family bible", "tags": []string{"sources"},
		"personIds": []int64{walter}, "mediaIds": []int64{media.ID}}, &tk, 201)
	ann.must("POST", fmt.Sprintf("%s/tasks/%d/claim", base, tk.ID), nil, nil, 200)
	ann.must("POST", fmt.Sprintf("%s/tasks/%d/comments", base, tk.ID), map[string]string{"bodyMd": "Bible is at **Aunt June's**."}, nil, 201)

	resp, err := admin.c.Get(e.ts.URL + "/api/v1/backups/download")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("download backup: %v", err)
	}
	archive, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Diverge: delete the task, the media and add another task.
	ann.must("DELETE", fmt.Sprintf("%s/tasks/%d", base, tk.ID), nil, nil, 204)
	ann.must("DELETE", fmt.Sprintf("/api/v1/media/%d", media.ID), nil, nil, 204)
	ann.must("POST", base+"/tasks", map[string]any{"title": "Interloper"}, nil, 201)

	var info struct{ Name string }
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "backup.tar.gz")
	fw.Write(archive)
	mw.Close()
	up, err := admin.c.Post(e.ts.URL+"/api/v1/backups/upload", mw.FormDataContentType(), &buf)
	if err != nil || up.StatusCode != 201 {
		t.Fatalf("upload backup: %v %d", err, up.StatusCode)
	}
	json.NewDecoder(up.Body).Decode(&info)
	up.Body.Close()
	admin.must("POST", "/api/v1/backups/"+info.Name+"/restore", map[string]string{"confirm": "RESTORE"}, nil, 200)

	var list []taskJSON
	admin.must("GET", base+"/tasks", nil, &list, 200)
	if len(list) != 1 {
		t.Fatalf("restored tasks: %+v", list)
	}
	got := list[0]
	if got.Title != "Scan the family bible" || got.Status != "in_progress" || got.ClaimedBy == nil || got.ClaimedBy.Username != "ann" ||
		len(got.People) != 1 || len(got.Media) != 1 || len(got.Tags) != 1 || got.CreatedBy.Username != "ann" {
		t.Fatalf("restored task: %+v", got)
	}
	var comments []commentJSON
	admin.must("GET", fmt.Sprintf("%s/tasks/%d/comments", base, got.ID), nil, &comments, 200)
	if len(comments) != 1 || comments[0].Author == nil || comments[0].Author.Username != "ann" || comments[0].BodyMD != "Bible is at **Aunt June's**." {
		t.Fatalf("restored comments: %+v", comments)
	}
	r, err := admin.c.Get(fmt.Sprintf("%s/api/v1/media/%d/file", e.ts.URL, got.Media[0].ID))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != 200 || !bytes.Equal(body, png) {
		t.Fatalf("restored task media file: %d", r.StatusCode)
	}
}

func TestQuickStatusRequiresClaim(t *testing.T) {
	e := start(t, t.TempDir())
	_, ann, bob, viewer, _ := setup(t, e)
	var tk taskJSON
	ann.must("POST", base+"/tasks", map[string]any{"title": "Visit the county archive"}, &tk, 201)
	path := fmt.Sprintf("%s/tasks/%d/status", base, tk.ID)

	var e1 apiErr
	if code, _ := ann.do("PATCH", path, map[string]string{"status": "in_progress"}, &e1); code != 409 || e1.Error.Code != "not_claimed" {
		t.Fatalf("unclaimed status change: %d %+v", code, e1)
	}
	ann.must("POST", fmt.Sprintf("%s/tasks/%d/claim", base, tk.ID), nil, nil, 200)

	// Any editor can move a claimed task between the quick statuses; attribution follows.
	var got taskJSON
	bob.must("PATCH", path, map[string]string{"status": "blocked"}, &got, 200)
	if got.Status != "blocked" || got.UpdatedBy.Username != "bob" || got.ClaimedBy.Username != "ann" {
		t.Fatalf("blocked: %+v", got)
	}
	ann.must("PATCH", path, map[string]string{"status": "done"}, &got, 200)
	if got.Status != "done" || got.UpdatedBy.Username != "ann" || got.ClaimedBy == nil {
		t.Fatalf("done keeps the claim as a record of who did it: %+v", got)
	}
	ann.must("PATCH", path, map[string]string{"status": "in_progress"}, &got, 200)

	// "open" is not a quick status (reopening = releasing the claim); unknown values fail.
	for _, st := range []string{"open", "nope", ""} {
		if code, _ := ann.do("PATCH", path, map[string]string{"status": st}, nil); code != 422 {
			t.Fatalf("status %q: want 422, got %d", st, code)
		}
	}
	viewer.must("PATCH", path, map[string]string{"status": "blocked"}, nil, 403)
	ann.must("PATCH", fmt.Sprintf("%s/tasks/999/status", base), map[string]string{"status": "blocked"}, nil, 404)

	// After release the task is unclaimed again, so quick changes are refused.
	ann.must("POST", fmt.Sprintf("%s/tasks/%d/unclaim", base, tk.ID), nil, nil, 200)
	ann.must("PATCH", path, map[string]string{"status": "blocked"}, nil, 409)

	// Changes are audited.
	var hist struct {
		Items []struct{ EntityType, Username string }
	}
	ann.must("GET", fmt.Sprintf("/api/v1/audit?entityType=task&entityId=%d", tk.ID), nil, &hist, 200)
	if len(hist.Items) < 5 {
		t.Fatalf("status changes should be audited: %+v", hist.Items)
	}
}

func TestComments(t *testing.T) {
	e := start(t, t.TempDir())
	admin, ann, bob, viewer, walter := setup(t, e)
	var tk taskJSON
	ann.must("POST", base+"/tasks", map[string]any{"title": "Trace the Hale farm", "personIds": []int64{walter}}, &tk, 201)
	path := fmt.Sprintf("%s/tasks/%d/comments", base, tk.ID)

	var list []commentJSON
	viewer.must("GET", path, nil, &list, 200)
	if len(list) != 0 {
		t.Fatal("new task should have no comments")
	}
	var c1, c2 commentJSON
	ann.must("POST", path, map[string]string{"bodyMd": "Tithe map shows *Hale* in 1840."}, &c1, 201)
	bob.must("POST", path, map[string]string{"bodyMd": "Land tax 1832 too."}, &c2, 201)
	if c1.Author.Username != "ann" || c1.Author.DisplayName != "ann" || c1.CreatedAt == "" {
		t.Fatalf("comment attribution: %+v", c1)
	}
	if code, _ := ann.do("POST", path, map[string]string{"bodyMd": "   "}, nil); code != 422 {
		t.Fatalf("empty comment: %d", code)
	}
	viewer.must("POST", path, map[string]string{"bodyMd": "hi"}, nil, 403)
	ann.must("POST", fmt.Sprintf("%s/tasks/999/comments", base), map[string]string{"bodyMd": "x"}, nil, 404)
	viewer.must("GET", fmt.Sprintf("%s/tasks/999/comments", base), nil, nil, 404)

	viewer.must("GET", path, nil, &list, 200)
	if len(list) != 2 || list[0].ID != c1.ID || list[1].ID != c2.ID {
		t.Fatalf("comments should be chronological: %+v", list)
	}
	var tasks []taskJSON
	viewer.must("GET", base+"/tasks", nil, &tasks, 200)
	if len(tasks) != 1 || tasks[0].CommentCount != 2 {
		t.Fatalf("commentCount: %+v", tasks)
	}

	// Only the author (or an admin) may edit or delete.
	one := fmt.Sprintf("%s/%d", path, c1.ID)
	var forbidden apiErr
	if code, _ := bob.do("PATCH", one, map[string]string{"bodyMd": "hijack"}, &forbidden); code != 403 || forbidden.Error.Code != "not_author" {
		t.Fatalf("non-author edit: %d %+v", code, forbidden)
	}
	bob.must("DELETE", one, nil, nil, 403)
	var edited commentJSON
	ann.must("PATCH", one, map[string]string{"bodyMd": "Tithe map shows *Hale* in 1841."}, &edited, 200)
	if edited.BodyMD != "Tithe map shows *Hale* in 1841." || !edited.Edited { // edited within the same second still counts
		t.Fatalf("edit: %+v", edited)
	}
	admin.must("DELETE", fmt.Sprintf("%s/%d", path, c2.ID), nil, nil, 204)
	ann.must("DELETE", fmt.Sprintf("%s/%d", path, c2.ID), nil, nil, 404)
	// A comment id from another task is not found under this task.
	var other taskJSON
	ann.must("POST", base+"/tasks", map[string]any{"title": "Other"}, &other, 201)
	ann.must("DELETE", fmt.Sprintf("%s/tasks/%d/comments/%d", base, other.ID, c1.ID), nil, nil, 404)

	// Research notes appear in the linked person's history.
	var hist struct{ Items []struct{ Summary string } }
	ann.must("GET", fmt.Sprintf("/api/v1/audit?personId=%d", walter), nil, &hist, 200)
	var notes int
	for _, h := range hist.Items {
		if strings.Contains(h.Summary, "research note") {
			notes++
		}
	}
	if notes != 4 { // 2 added, 1 edited, 1 deleted
		t.Fatalf("history notes: %+v", hist.Items)
	}

	// Deleting the task deletes its comments.
	ann.must("DELETE", fmt.Sprintf("%s/tasks/%d", base, tk.ID), nil, nil, 204)
	var n int
	if err := e.db.PluginDB().QueryRow(context.Background(), `SELECT COUNT(*) FROM plg_task_comments`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("comments should cascade: n=%d err=%v", n, err)
	}
}

// An install created with plugin schema v1 upgrades to v2 (comments) on start.
func TestUpgradeFromSchemaV1(t *testing.T) {
	dir := t.TempDir()
	e := start(t, dir)
	_, ann, _, _, _ := setup(t, e)
	var tk taskJSON
	ann.must("POST", base+"/tasks", map[string]any{"title": "Made before comments existed"}, &tk, 201)
	ctx := context.Background()
	// Roll the database back to plugin schema v1.
	db := e.db.PluginDB()
	if err := db.WriteTx(ctx, store.SystemActor, func(tx *sql.Tx, _ func(store.AuditEntry) error) error {
		for _, q := range []string{`DROP TABLE plg_task_comments`, `DELETE FROM plugin_migration WHERE plugin_id = 'task' AND version = 2`} {
			if _, err := tx.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	e.stop()

	e2 := start(t, dir)
	ann2 := e2.login("ann", "password123")
	var list []taskJSON
	ann2.must("GET", base+"/tasks", nil, &list, 200)
	if len(list) != 1 || list[0].CommentCount != 0 {
		t.Fatalf("after upgrade: %+v", list)
	}
	ann2.must("POST", fmt.Sprintf("%s/tasks/%d/comments", base, tk.ID), map[string]string{"bodyMd": "Works after upgrade"}, nil, 201)
}
