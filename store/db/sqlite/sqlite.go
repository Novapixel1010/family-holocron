// Package sqlite implements store.Driver on top of the pure-Go modernc.org/sqlite driver.
//
// The database runs in WAL mode with foreign keys enforced. Write transactions use
// BEGIN IMMEDIATE (via _txlock) so concurrent writers queue on the busy timeout
// rather than failing on lock upgrade.
package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/novapixel1010/family-holocron/store"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

//go:embed migration/*.sql
var migrationFS embed.FS

// DB is the SQLite store driver.
type DB struct {
	db       *sql.DB
	path     string
	mu       sync.RWMutex
	listener func(store.Change)
}

var _ store.Driver = (*DB)(nil)

// DSN builds the connection string for path.
func DSN(path string) string {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(10000)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Set("_txlock", "immediate")
	return "file:" + path + "?" + q.Encode()
}

// Open opens (creating if needed) the database at path and applies migrations.
func Open(ctx context.Context, path string) (*DB, error) {
	db, err := sql.Open("sqlite", DSN(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)
	db.SetConnMaxIdleTime(5 * time.Minute)
	d := &DB{db: db, path: path}
	if err := d.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := d.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return d, nil
}

// Close closes the database.
func (d *DB) Close() error { return d.db.Close() }

// Ping checks the database connection.
func (d *DB) Ping(ctx context.Context) error {
	var n int
	return d.db.QueryRowContext(ctx, "SELECT 1").Scan(&n)
}

// SetChangeListener implements store.Driver.
func (d *DB) SetChangeListener(fn func(store.Change)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.listener = fn
}

// LatestSchemaVersion is the highest embedded migration version.
func LatestSchemaVersion() int {
	ms, _ := loadMigrations()
	if len(ms) == 0 {
		return 0
	}
	return ms[len(ms)-1].version
}

type migration struct {
	version int
	name    string
	sql     string
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFS, "migration")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		v, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil {
			return nil, fmt.Errorf("bad migration name %q", name)
		}
		b, err := migrationFS.ReadFile("migration/" + name)
		if err != nil {
			return nil, err
		}
		out = append(out, migration{version: v, name: name, sql: string(b)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func (d *DB) migrate(ctx context.Context) error {
	if _, err := d.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migration (
		version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at INTEGER NOT NULL)`); err != nil {
		return err
	}
	current, err := d.SchemaVersion(ctx)
	if err != nil {
		return err
	}
	ms, err := loadMigrations()
	if err != nil {
		return err
	}
	if len(ms) > 0 && current > ms[len(ms)-1].version {
		return fmt.Errorf("database schema version %d is newer than this build supports (%d)", current, ms[len(ms)-1].version)
	}
	for _, m := range ms {
		if m.version <= current {
			continue
		}
		tx, err := d.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", m.name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migration (version, name, applied_at) VALUES (?, ?, ?)`,
			m.version, m.name, time.Now().Unix()); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// SchemaVersion returns the applied core schema version.
func (d *DB) SchemaVersion(ctx context.Context) (int, error) {
	var v sql.NullInt64
	err := d.db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migration`).Scan(&v)
	if err != nil {
		return 0, err
	}
	return int(v.Int64), nil
}

// tx wraps *sql.Tx and collects change notifications to emit after commit.
type tx struct {
	*sql.Tx
	changes []store.Change
}

func (d *DB) withTx(ctx context.Context, fn func(t *tx) error) error {
	sqlTx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	t := &tx{Tx: sqlTx}
	if err := fn(t); err != nil {
		sqlTx.Rollback()
		return err
	}
	if err := sqlTx.Commit(); err != nil {
		return err
	}
	d.mu.RLock()
	l := d.listener
	d.mu.RUnlock()
	if l != nil {
		for _, c := range t.changes {
			l(c)
		}
	}
	return nil
}

// audit writes an audit log entry inside the transaction.
func (t *tx) audit(ctx context.Context, actor store.Actor, e store.AuditEntry) error {
	changes := e.Changes
	if changes == nil {
		changes = map[string]store.FieldChange{}
	}
	cj, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	pids := uniqueIDs(e.PersonIDs)
	pj, _ := json.Marshal(pids)
	res, err := t.ExecContext(ctx, `INSERT INTO audit_log
		(created_at, user_id, username, action, entity_type, entity_id, entity_label, person_ids, changes, summary)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		now(), nullID(actor.UserID), actor.Username, e.Action, e.EntityType, e.EntityID, e.EntityLabel,
		string(pj), string(cj), e.Summary)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	for _, pid := range pids {
		if _, err := t.ExecContext(ctx, `INSERT OR IGNORE INTO audit_person (audit_id, person_id) VALUES (?, ?)`, id, pid); err != nil {
			return err
		}
	}
	t.changes = append(t.changes, store.Change{Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID, Actor: actor, PersonIDs: pids})
	return nil
}

func uniqueIDs(ids []int64) []int64 {
	seen := map[int64]bool{}
	out := []int64{}
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func now() int64 { return time.Now().Unix() }

func unix(v int64) time.Time { return time.Unix(v, 0).UTC() }

func nullID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

func nullIDPtr(id *int64) any {
	if id == nil || *id <= 0 {
		return nil
	}
	return *id
}

func newUID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func isFK(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrNotFound
	}
	return err
}

// placeholders returns "?, ?, ?" for n values and the args slice.
func inClause[T any](vals []T) (string, []any) {
	ph := make([]string, len(vals))
	args := make([]any, len(vals))
	for i, v := range vals {
		ph[i] = "?"
		args[i] = v
	}
	return strings.Join(ph, ", "), args
}

func likeEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(s) + "%"
}

// --- attribution helpers -------------------------------------------------

// attrSelect returns the column list that loads attribution for table alias a.
func attrSelect(a string) string {
	return fmt.Sprintf(`%[1]s.created_at, %[1]s.updated_at,
		%[1]s_cu.id, %[1]s_cu.username, %[1]s_cu.display_name,
		%[1]s_uu.id, %[1]s_uu.username, %[1]s_uu.display_name`, a)
}

// attrJoin returns the LEFT JOINs for attrSelect.
func attrJoin(a string) string {
	return fmt.Sprintf(` LEFT JOIN user %[1]s_cu ON %[1]s_cu.id = %[1]s.created_by_user_id
		LEFT JOIN user %[1]s_uu ON %[1]s_uu.id = %[1]s.updated_by_user_id `, a)
}

type attrScan struct {
	created, updated int64
	cID, uID         sql.NullInt64
	cName, uName     sql.NullString
	cDisp, uDisp     sql.NullString
}

func (s *attrScan) dest() []any {
	return []any{&s.created, &s.updated, &s.cID, &s.cName, &s.cDisp, &s.uID, &s.uName, &s.uDisp}
}

func (s *attrScan) value() store.Attribution {
	a := store.Attribution{CreatedAt: unix(s.created), UpdatedAt: unix(s.updated)}
	if s.cID.Valid {
		a.CreatedBy = &store.UserRef{ID: s.cID.Int64, Username: s.cName.String, DisplayName: s.cDisp.String}
	}
	if s.uID.Valid {
		a.UpdatedBy = &store.UserRef{ID: s.uID.Int64, Username: s.uName.String, DisplayName: s.uDisp.String}
	}
	return a
}

type rowScanner interface{ Scan(dest ...any) error }
