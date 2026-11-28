package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/novapixel1010/family-holocron/store"
)

func newTestDB(t *testing.T) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

func mustUser(t *testing.T, db *DB, name string, role store.Role) *store.User {
	t.Helper()
	u, err := db.CreateUser(context.Background(), store.SystemActor, &store.User{Username: name, DisplayName: name + " Display", PasswordHash: "x", Role: role})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func TestPragmas(t *testing.T) {
	db, _ := newTestDB(t)
	ctx := context.Background()
	var mode string
	var fk int
	if err := db.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if err := db.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" || fk != 1 {
		t.Fatalf("journal_mode=%s foreign_keys=%d", mode, fk)
	}
	v, _ := db.SchemaVersion(ctx)
	if v != LatestSchemaVersion() || v < 1 {
		t.Fatalf("schema version %d", v)
	}
}

func TestFirstAdminOnlyOnce(t *testing.T) {
	db, _ := newTestDB(t)
	ctx := context.Background()
	if _, err := db.CreateFirstAdmin(ctx, &store.User{Username: "root", PasswordHash: "x"}); err != nil {
		t.Fatal(err)
	}
	_, err := db.CreateFirstAdmin(ctx, &store.User{Username: "other", PasswordHash: "x"})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("second setup: want conflict, got %v", err)
	}
}

func TestPersonPersistenceAndAttribution(t *testing.T) {
	db, path := newTestDB(t)
	ctx := context.Background()
	alice := mustUser(t, db, "alice", store.RoleEditor)
	bob := mustUser(t, db, "bob", store.RoleEditor)

	p, err := db.CreatePerson(ctx, alice.Actor(), store.PersonInput{
		GivenName: "Ada", Surname: "Lovelace", Sex: "f", BirthDate: "10 DEC 1815", BirthPlace: "London",
		About: "# Ada\nFirst *programmer*.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.CreatedBy == nil || p.CreatedBy.Username != "alice" || p.UpdatedBy.Username != "alice" {
		t.Fatalf("attribution on create: %+v / %+v", p.CreatedBy, p.UpdatedBy)
	}
	if p.Sex != "F" || p.BirthSort != "1815-12-10" {
		t.Fatalf("normalisation: sex=%s sort=%s", p.Sex, p.BirthSort)
	}

	in := store.PersonInput{GivenName: "Ada", Surname: "King", Sex: "F", BirthDate: "1815-12-10", About: p.About, DeathDate: "1852"}
	p2, err := db.UpdatePerson(ctx, bob.Actor(), p.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	if p2.CreatedBy.Username != "alice" || p2.UpdatedBy.Username != "bob" {
		t.Fatalf("attribution on update: created=%s updated=%s", p2.CreatedBy.Username, p2.UpdatedBy.Username)
	}
	if !p2.Deceased {
		t.Fatal("death date should imply deceased")
	}

	// Persistence across reopen.
	db.Close()
	db2, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	got, err := db2.GetPerson(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Surname != "King" || got.About != p.About || got.UpdatedBy.Username != "bob" {
		t.Fatalf("after reopen: %+v", got)
	}

	// Audit trail.
	entries, err := db2.ListAudit(ctx, store.FindAudit{PersonID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 audit entries, got %d", len(entries))
	}
	upd := entries[0]
	if upd.Action != store.ActionUpdate || upd.User.Username != "bob" || upd.Username != "bob" {
		t.Fatalf("update audit: %+v", upd)
	}
	if c, ok := upd.Changes["surname"]; !ok || c.From != "Lovelace" || c.To != "King" {
		t.Fatalf("surname diff: %+v", upd.Changes)
	}
	if _, ok := upd.Changes["about"]; ok {
		t.Fatal("unchanged field should not appear in diff")
	}
	if entries[1].Action != store.ActionCreate || entries[1].User.Username != "alice" {
		t.Fatalf("create audit: %+v", entries[1])
	}
}

func TestAttributionSurvivesUserDisable(t *testing.T) {
	db, _ := newTestDB(t)
	ctx := context.Background()
	mustUser(t, db, "admin", store.RoleAdmin)
	carol := mustUser(t, db, "carol", store.RoleEditor)
	p, _ := db.CreatePerson(ctx, carol.Actor(), store.PersonInput{GivenName: "X"})
	disabled := store.UserDisabled
	if _, err := db.UpdateUser(ctx, store.SystemActor, carol.ID, store.UpdateUser{Status: &disabled}); err != nil {
		t.Fatal(err)
	}
	got, _ := db.GetPerson(ctx, p.ID)
	if got.CreatedBy == nil || got.CreatedBy.Username != "carol" {
		t.Fatal("attribution lost")
	}
}

func TestLastAdminProtected(t *testing.T) {
	db, _ := newTestDB(t)
	ctx := context.Background()
	a := mustUser(t, db, "admin", store.RoleAdmin)
	viewer := store.RoleViewer
	if _, err := db.UpdateUser(ctx, a.Actor(), a.ID, store.UpdateUser{Role: &viewer}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want conflict demoting last admin, got %v", err)
	}
}

func TestRelationshipsAndCycles(t *testing.T) {
	db, _ := newTestDB(t)
	ctx := context.Background()
	u := mustUser(t, db, "uu", store.RoleEditor)
	mk := func(n string) *store.Person {
		p, err := db.CreatePerson(ctx, u.Actor(), store.PersonInput{GivenName: n})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	gp, parent, child := mk("Grandparent"), mk("Parent"), mk("Child")
	if _, err := db.CreateRelationship(ctx, u.Actor(), store.RelationshipInput{Type: "parent", Person1ID: gp.ID, Person2ID: parent.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateRelationship(ctx, u.Actor(), store.RelationshipInput{Type: "parent", Person1ID: parent.ID, Person2ID: child.ID}); err != nil {
		t.Fatal(err)
	}
	// Child cannot become grandparent's parent.
	_, err := db.CreateRelationship(ctx, u.Actor(), store.RelationshipInput{Type: "parent", Person1ID: child.ID, Person2ID: gp.ID})
	var ve *store.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want validation error for cycle, got %v", err)
	}
	// Duplicate.
	_, err = db.CreateRelationship(ctx, u.Actor(), store.RelationshipInput{Type: "parent", Person1ID: parent.ID, Person2ID: child.ID})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want conflict for duplicate, got %v", err)
	}
	// Spouse normalisation.
	sp := mk("Spouse")
	r, err := db.CreateRelationship(ctx, u.Actor(), store.RelationshipInput{Type: "spouse", Person1ID: sp.ID, Person2ID: parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	if r.Person1ID != parent.ID || r.Subtype != "married" {
		t.Fatalf("spouse normalisation: %+v", r)
	}
	if r.CreatedBy == nil || r.CreatedBy.ID != u.ID {
		t.Fatal("relationship attribution missing")
	}
	// Deleting a person cascades relationships.
	if err := db.DeletePerson(ctx, u.Actor(), parent.ID); err != nil {
		t.Fatal(err)
	}
	rels, _ := db.ListRelationships(ctx, store.FindRelationship{})
	if len(rels) != 0 {
		t.Fatalf("relationships should cascade, have %d", len(rels))
	}
	// The deletion is visible in the child's history (it concerned them).
	hist, _ := db.ListAudit(ctx, store.FindAudit{PersonID: &child.ID})
	if len(hist) == 0 || hist[0].Action != store.ActionDelete || hist[0].EntityType != store.EntityPerson {
		t.Fatalf("child history: %+v", hist)
	}
}

func TestEventsNotesMediaAttribution(t *testing.T) {
	db, _ := newTestDB(t)
	ctx := context.Background()
	u := mustUser(t, db, "uu", store.RoleEditor)
	v := mustUser(t, db, "vv", store.RoleEditor)
	a, _ := db.CreatePerson(ctx, u.Actor(), store.PersonInput{GivenName: "A"})
	b, _ := db.CreatePerson(ctx, u.Actor(), store.PersonInput{GivenName: "B"})

	e, err := db.CreateEvent(ctx, u.Actor(), store.EventInput{Type: "marriage", Date: "ABT 1900", Place: "Paris",
		Participants: []store.EventParticipant{{PersonID: a.ID}, {PersonID: b.ID, Role: "spouse"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Participants) != 2 || e.SortDate != "1900-00-00" || e.CreatedBy.Username != "uu" {
		t.Fatalf("event: %+v", e)
	}
	e2, err := db.UpdateEvent(ctx, v.Actor(), e.ID, store.EventInput{Type: "marriage", Date: "1901", Participants: e.Participants})
	if err != nil {
		t.Fatal(err)
	}
	if e2.UpdatedBy.Username != "vv" || e2.CreatedBy.Username != "uu" {
		t.Fatal("event attribution")
	}
	if _, err := db.CreateEvent(ctx, u.Actor(), store.EventInput{Type: "residence", Date: "1850"}); err == nil {
		t.Fatal("person event without participants should fail")
	}
	if _, err := db.CreateEvent(ctx, u.Actor(), store.EventInput{Type: "historical", Title: "Great War begins", Date: "1914-07-28"}); err != nil {
		t.Fatal(err)
	}
	list, _ := db.ListEvents(ctx, store.FindEvent{PersonID: &a.ID})
	if len(list) != 1 {
		t.Fatalf("events for person: %d", len(list))
	}
	all, _ := db.ListEvents(ctx, store.FindEvent{From: "1900", To: "1910"})
	if len(all) != 1 {
		t.Fatalf("date range filter: %d", len(all))
	}

	n, err := db.CreateNote(ctx, u.Actor(), a.ID, store.NoteInput{Title: "Census", Body: "Found in **1901** census"})
	if err != nil {
		t.Fatal(err)
	}
	n2, err := db.UpdateNote(ctx, v.Actor(), n.ID, store.NoteInput{Title: "Census 1901", Body: n.Body})
	if err != nil || n2.UpdatedBy.Username != "vv" || n2.CreatedBy.Username != "uu" {
		t.Fatalf("note attribution: %v %+v", err, n2)
	}

	m, err := db.CreateMedia(ctx, u.Actor(), &store.Media{StoragePath: "ab/abc.jpg", OriginalName: "abc.jpg", MimeType: "image/jpeg", SizeBytes: 3, PersonIDs: []int64{a.ID, a.ID, b.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.PersonIDs) != 2 || m.CreatedBy.Username != "uu" {
		t.Fatalf("media: %+v", m)
	}
	m2, err := db.UpdateMedia(ctx, v.Actor(), m.ID, store.MediaInput{Title: "Wedding", PersonIDs: []int64{b.ID}})
	if err != nil || m2.UpdatedBy.Username != "vv" || len(m2.PersonIDs) != 1 {
		t.Fatalf("media update: %v %+v", err, m2)
	}

	// Each entity type has created + updated audit rows by the right users.
	for _, et := range []string{store.EntityEvent, store.EntityNote, store.EntityMedia} {
		rows, _ := db.ListAudit(ctx, store.FindAudit{EntityType: et})
		var sawCreateU, sawUpdateV bool
		for _, r := range rows {
			if r.Action == store.ActionCreate && r.Username == "uu" {
				sawCreateU = true
			}
			if r.Action == store.ActionUpdate && r.Username == "vv" {
				sawUpdateV = true
			}
		}
		if !sawCreateU || !sawUpdateV {
			t.Fatalf("%s audit incomplete: %+v", et, rows)
		}
	}

	// Deleting A removes the event only A was in? Marriage involves A and B, so it stays.
	if err := db.DeletePerson(ctx, u.Actor(), a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetEvent(ctx, e.ID); err != nil {
		t.Fatal("shared event should survive")
	}
	notes, _ := db.ListNotes(ctx, a.ID)
	if len(notes) != 0 {
		t.Fatal("notes should cascade")
	}
}

func TestConcurrentWrites(t *testing.T) {
	db, _ := newTestDB(t)
	ctx := context.Background()
	u := mustUser(t, db, "uu", store.RoleEditor)
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := db.CreatePerson(ctx, u.Actor(), store.PersonInput{GivenName: "P", Surname: "Concurrent"})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	_, total, _ := db.ListPersons(ctx, store.FindPerson{Query: "concurrent"})
	if total != 40 {
		t.Fatalf("want 40, got %d", total)
	}
}

func TestSnapshotAndRestore(t *testing.T) {
	db, _ := newTestDB(t)
	ctx := context.Background()
	u := mustUser(t, db, "admin", store.RoleAdmin)
	p, _ := db.CreatePerson(ctx, u.Actor(), store.PersonInput{GivenName: "Before", About: "kept"})
	if err := db.ApplyPluginMigrations(ctx, "demo", []store.PluginMigration{{Version: 1, SQL: `CREATE TABLE plg_demo_x (v TEXT)`}}); err != nil {
		t.Fatal(err)
	}
	db.db.ExecContext(ctx, `INSERT INTO plg_demo_x VALUES ('from-backup')`)

	snap := filepath.Join(t.TempDir(), "snap.db")
	if err := db.SnapshotTo(ctx, snap); err != nil {
		t.Fatal(err)
	}
	// Diverge after the snapshot.
	db.CreatePerson(ctx, u.Actor(), store.PersonInput{GivenName: "After"})
	db.db.ExecContext(ctx, `DELETE FROM plg_demo_x`)
	token := "tok"
	db.CreateSession(ctx, &store.Session{TokenHash: token, UserID: u.ID, CreatedAt: unix(now()), ExpiresAt: unix(now() + 3600), LastSeenAt: unix(now())})
	db.CreateSession(ctx, &store.Session{TokenHash: "other", UserID: u.ID, CreatedAt: unix(now()), ExpiresAt: unix(now() + 3600), LastSeenAt: unix(now())})

	// Restore needs a migrated source; reopen it through Open.
	src, err := Open(ctx, snap)
	if err != nil {
		t.Fatal(err)
	}
	src.Close()
	if err := db.RestoreFrom(ctx, snap, store.KeepSession{TokenHash: token, UserID: u.ID, Username: "admin"}); err != nil {
		t.Fatal(err)
	}
	_, total, _ := db.ListPersons(ctx, store.FindPerson{})
	if total != 1 {
		t.Fatalf("want 1 person after restore, got %d", total)
	}
	got, err := db.GetPerson(ctx, p.ID)
	if err != nil || got.About != "kept" || got.CreatedBy.Username != "admin" {
		t.Fatalf("restored person: %v %+v", err, got)
	}
	var v string
	db.db.QueryRowContext(ctx, `SELECT v FROM plg_demo_x`).Scan(&v)
	if v != "from-backup" {
		t.Fatalf("plugin table not restored: %q", v)
	}
	if _, _, err := db.GetSession(ctx, token); err != nil {
		t.Fatalf("restoring admin's session should survive: %v", err)
	}
	if _, _, err := db.GetSession(ctx, "other"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("other sessions should be dropped")
	}
	// New inserts must not collide with restored ids.
	np, err := db.CreatePerson(ctx, u.Actor(), store.PersonInput{GivenName: "New"})
	if err != nil || np.ID <= p.ID {
		t.Fatalf("post-restore insert: %v id=%d", err, np.ID)
	}
}

func TestImportBatch(t *testing.T) {
	db, _ := newTestDB(t)
	ctx := context.Background()
	u := mustUser(t, db, "uu", store.RoleEditor)
	res, err := db.Import(ctx, u.Actor(), &store.ImportBatch{
		Source: "test.ged",
		Persons: []store.ImportPerson{
			{Ref: "@I1@", Input: store.PersonInput{GivenName: "Dad"}},
			{Ref: "@I2@", Input: store.PersonInput{GivenName: "Kid"}},
			{Ref: "@I3@", Input: store.PersonInput{}}, // invalid: no name
		},
		Relationships: []store.ImportRelationship{{Type: "parent", Person1Ref: "@I1@", Person2Ref: "@I2@"}, {Type: "parent", Person1Ref: "@I1@", Person2Ref: "@I9@"}},
		Events:        []store.ImportEvent{{Input: store.EventInput{Type: "residence", Date: "1900"}, ParticipantRefs: []string{"@I2@"}}},
		Notes:         []store.ImportNote{{PersonRef: "@I1@", Input: store.NoteInput{Body: "hello"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Persons != 2 || res.Relationships != 1 || res.Events != 1 || res.Notes != 1 || len(res.Warnings) != 2 {
		t.Fatalf("import result: %+v", res)
	}
	people, _, _ := db.ListPersons(ctx, store.FindPerson{})
	for _, p := range people {
		if p.CreatedBy == nil || p.CreatedBy.Username != "uu" {
			t.Fatal("imported records must be attributed to the importing user")
		}
	}
}

// A backup taken before a plugin existed must not erase the record of the plugin's
// applied migrations: its tables still exist after restore, so re-running the
// migration on the next start would fail with "table already exists".
func TestRestoreKeepsPluginMigrationState(t *testing.T) {
	db, path := newTestDB(t)
	ctx := context.Background()
	u := mustUser(t, db, "admin", store.RoleAdmin)
	snap := filepath.Join(t.TempDir(), "before-plugin.db")
	if err := db.SnapshotTo(ctx, snap); err != nil { // backup predates the plugin
		t.Fatal(err)
	}
	migs := []store.PluginMigration{{Version: 1, SQL: `CREATE TABLE plg_demo_item (id INTEGER PRIMARY KEY, v TEXT)`}}
	if err := db.ApplyPluginMigrations(ctx, "demo", migs); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareRestoreSource(ctx, snap); err != nil {
		t.Fatal(err)
	}
	if err := db.RestoreFrom(ctx, snap, store.KeepSession{UserID: u.ID, Username: "admin"}); err != nil {
		t.Fatal(err)
	}
	db.Close()
	// Simulate the next start-up: reopen and apply plugin migrations again.
	db2, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	if err := db2.ApplyPluginMigrations(ctx, "demo", migs); err != nil {
		t.Fatalf("plugin migration after restore: %v", err)
	}
	var n int
	if err := db2.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM plg_demo_item`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("plugin table after restore: n=%d err=%v", n, err)
	}
}

func TestBioTemplateSeedingIsOneTime(t *testing.T) {
	db, path := newTestDB(t)
	ctx := context.Background()
	if err := db.SeedBioTemplates(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.SeedBioTemplates(ctx); err != nil { // idempotent
		t.Fatal(err)
	}
	list, _ := db.ListBioTemplates(ctx)
	if len(list) != 1 || list[0].Name != "Standard Biography" || !strings.Contains(list[0].ContentMD, "## Places Lived") ||
		!strings.Contains(list[0].ContentMD, "## Favorite Food") || !strings.Contains(list[0].ContentMD, "## Memorable Stories") {
		t.Fatalf("seeded templates: %+v", list)
	}
	// An admin deleting every template must stick across restarts.
	admin := mustUser(t, db, "admin", store.RoleAdmin)
	if err := db.DeleteBioTemplate(ctx, admin.Actor(), list[0].ID); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db2, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	if err := db2.SeedBioTemplates(ctx); err != nil {
		t.Fatal(err)
	}
	if list, _ := db2.ListBioTemplates(ctx); len(list) != 0 {
		t.Fatalf("deleted starter template came back: %+v", list)
	}
	// A database that already has templates (e.g. created before seeding) is not touched.
	db3, _ := newTestDB(t)
	a3 := mustUser(t, db3, "admin", store.RoleAdmin)
	if _, err := db3.CreateBioTemplate(ctx, a3.Actor(), store.BioTemplateInput{Name: "Mine", ContentMD: "x"}); err != nil {
		t.Fatal(err)
	}
	db3.SeedBioTemplates(ctx)
	if list, _ := db3.ListBioTemplates(ctx); len(list) != 1 || list[0].Name != "Mine" {
		t.Fatalf("seed must not add to an existing list: %+v", list)
	}
}

func TestMustChangePasswordRequiresLocalPassword(t *testing.T) {
	db, _ := newTestDB(t)
	ctx := context.Background()
	u := mustUser(t, db, "local", store.RoleEditor)
	yes := true
	got, err := db.UpdateUser(ctx, store.SystemActor, u.ID, store.UpdateUser{MustChangePassword: &yes})
	if err != nil || !got.MustChangePassword {
		t.Fatalf("flag local account: %v %+v", err, got)
	}
	// The database itself refuses the flag on an account without a local password.
	if _, err := db.db.ExecContext(ctx, `UPDATE user SET password_hash = '' WHERE id = ?`, u.ID); err == nil || !strings.Contains(err.Error(), "CHECK") {
		t.Fatalf("CHECK constraint should reject emptying the hash while flagged: %v", err)
	}
	// Session lookups carry the flag (the middleware relies on it).
	db.CreateSession(ctx, &store.Session{TokenHash: "t", UserID: u.ID, CreatedAt: unix(now()), ExpiresAt: unix(now() + 60), LastSeenAt: unix(now())})
	_, su, err := db.GetSession(ctx, "t")
	if err != nil || !su.MustChangePassword || !su.HasLocalPassword {
		t.Fatalf("session user: %v %+v", err, su)
	}
}
