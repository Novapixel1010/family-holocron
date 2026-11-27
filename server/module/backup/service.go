// Package backup creates, validates and restores full-instance backup archives
// (database snapshot + portable JSON export + media files).
package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/novapixel1010/family-holocron/internal/version"
	"github.com/novapixel1010/family-holocron/store"
)

// Archive member names.
const (
	ManifestFile = "manifest.json"
	DBFile       = "holocron.db"
	ExportFile   = "export.json"
	MediaPrefix  = "media/"
)

// Backup kinds.
const (
	KindManual = "manual"
	KindSafety = "safety" // automatic pre-restore snapshot
	KindUpload = "upload"
)

// FileInfo records the size and checksum of one archive member.
type FileInfo struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Manifest describes a backup archive.
type Manifest struct {
	Format        int                 `json:"format"`
	AppVersion    string              `json:"appVersion"`
	SchemaVersion int                 `json:"schemaVersion"`
	CreatedAt     time.Time           `json:"createdAt"`
	CreatedBy     string              `json:"createdBy"`
	Kind          string              `json:"kind"`
	Counts        store.Stats         `json:"counts"`
	Files         map[string]FileInfo `json:"files"`
}

// Info is a stored backup as listed by the API.
type Info struct {
	Name      string    `json:"name"`
	SizeBytes int64     `json:"sizeBytes"`
	CreatedAt time.Time `json:"createdAt"`
	Kind      string    `json:"kind"`
	Manifest  *Manifest `json:"manifest"`
}

// PrepareFunc validates and migrates an extracted database file in place,
// returning its original schema version.
type PrepareFunc func(ctx context.Context, dbPath string) (int, error)

// Service implements backup operations.
type Service struct {
	Store         *store.Store
	MediaDir      string
	BackupDir     string
	SchemaVersion func() int
	Prepare       PrepareFunc
	Logger        *slog.Logger
	// OnRestored runs after a successful restore (e.g. reload plugin state).
	OnRestored func(ctx context.Context)

	mu sync.Mutex // serialises backup and restore operations
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,120}\.tar\.gz$`)

// ValidName reports whether name is an acceptable stored backup file name.
func ValidName(name string) bool { return nameRe.MatchString(name) && !strings.Contains(name, "..") }

func (s *Service) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// ---- create --------------------------------------------------------------------

// Write produces a complete backup archive on w.
func (s *Service) Write(ctx context.Context, actor store.Actor, kind string, w io.Writer) (*Manifest, error) {
	work, err := os.MkdirTemp(s.BackupDir, ".work-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)

	dbPath := filepath.Join(work, DBFile)
	if err := s.Store.SnapshotTo(ctx, dbPath); err != nil {
		return nil, fmt.Errorf("snapshot database: %w", err)
	}
	export, err := BuildExport(ctx, s.Store)
	if err != nil {
		return nil, fmt.Errorf("build export: %w", err)
	}
	exportPath := filepath.Join(work, ExportFile)
	if err := writeJSONFile(exportPath, export); err != nil {
		return nil, err
	}
	stats, err := s.Store.Stats(ctx)
	if err != nil {
		return nil, err
	}
	m := &Manifest{
		Format: version.BackupFormat, AppVersion: version.Version, SchemaVersion: s.SchemaVersion(),
		CreatedAt: time.Now().UTC(), CreatedBy: actor.Username, Kind: kind, Counts: *stats, Files: map[string]FileInfo{},
	}

	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	add := func(name, src string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o640, Size: st.Size(), ModTime: st.ModTime(), Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		h := sha256.New()
		n, err := io.Copy(io.MultiWriter(tw, h), f)
		if err != nil {
			return err
		}
		m.Files[name] = FileInfo{Size: n, SHA256: hex.EncodeToString(h.Sum(nil))}
		return nil
	}
	if err := add(DBFile, dbPath); err != nil {
		return nil, err
	}
	if err := add(ExportFile, exportPath); err != nil {
		return nil, err
	}
	err = filepath.WalkDir(s.MediaDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if strings.HasPrefix(d.Name(), ".") && p != s.MediaDir {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(s.MediaDir, p)
		if err != nil {
			return err
		}
		return add(MediaPrefix+filepath.ToSlash(rel), p)
	})
	if err != nil {
		return nil, fmt.Errorf("archive media: %w", err)
	}
	mj, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := tw.WriteHeader(&tar.Header{Name: ManifestFile, Mode: 0o640, Size: int64(len(mj)), ModTime: m.CreatedAt, Typeflag: tar.TypeReg}); err != nil {
		return nil, err
	}
	if _, err := tw.Write(mj); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return m, nil
}

// CreateStored writes a backup into the backup directory and returns its info.
func (s *Service) CreateStored(ctx context.Context, actor store.Actor, kind, prefix string) (*Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := s.createStoredLocked(ctx, actor, kind, prefix)
	if err != nil {
		return nil, err
	}
	if kind == KindManual {
		_ = s.Store.RecordAudit(ctx, actor, store.AuditEntry{Action: store.ActionBackup, EntityType: store.EntitySystem,
			EntityLabel: info.Name, Summary: "Created backup " + info.Name})
	}
	return info, nil
}

func (s *Service) createStoredLocked(ctx context.Context, actor store.Actor, kind, prefix string) (*Info, error) {
	name := s.uniqueName(prefix)
	tmp, err := os.CreateTemp(s.BackupDir, ".partial-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	m, err := s.Write(ctx, actor, kind, tmp)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}
	final := filepath.Join(s.BackupDir, name)
	if err := os.Rename(tmp.Name(), final); err != nil {
		return nil, err
	}
	if err := writeJSONFile(final+".json", m); err != nil {
		return nil, err
	}
	return s.Get(name)
}

func (s *Service) uniqueName(prefix string) string {
	base := prefix + time.Now().UTC().Format("20060102-150405")
	name := base + ".tar.gz"
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(s.BackupDir, name)); errors.Is(err, fs.ErrNotExist) {
			return name
		}
		name = fmt.Sprintf("%s-%d.tar.gz", base, i)
	}
}

// ---- list / get / delete -------------------------------------------------------

// List returns stored backups, newest first.
func (s *Service) List() ([]*Info, error) {
	entries, err := os.ReadDir(s.BackupDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []*Info{}, nil
		}
		return nil, err
	}
	out := []*Info{}
	for _, e := range entries {
		if e.IsDir() || !ValidName(e.Name()) {
			continue
		}
		info, err := s.Get(e.Name())
		if err != nil {
			continue
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// Get returns info for a stored backup.
func (s *Service) Get(name string) (*Info, error) {
	if !ValidName(name) {
		return nil, store.ErrNotFound
	}
	p := filepath.Join(s.BackupDir, name)
	st, err := os.Stat(p)
	if err != nil {
		return nil, store.ErrNotFound
	}
	info := &Info{Name: name, SizeBytes: st.Size(), CreatedAt: st.ModTime().UTC(), Kind: KindManual}
	if b, err := os.ReadFile(p + ".json"); err == nil {
		var m Manifest
		if json.Unmarshal(b, &m) == nil {
			info.Manifest = &m
			info.CreatedAt = m.CreatedAt
			if m.Kind != "" {
				info.Kind = m.Kind
			}
		}
	}
	return info, nil
}

// Path returns the filesystem path of a stored backup.
func (s *Service) Path(name string) (string, error) {
	if _, err := s.Get(name); err != nil {
		return "", err
	}
	return filepath.Join(s.BackupDir, name), nil
}

// Delete removes a stored backup.
func (s *Service) Delete(ctx context.Context, actor store.Actor, name string) error {
	p, err := s.Path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		return err
	}
	_ = os.Remove(p + ".json")
	return s.Store.RecordAudit(ctx, actor, store.AuditEntry{Action: store.ActionDelete, EntityType: store.EntitySystem,
		EntityLabel: name, Summary: "Deleted backup " + name})
}

// ---- validate / upload ------------------------------------------------------------

// Extract unpacks archive into dest, verifying structure and checksums.
func Extract(archive io.Reader, dest string, maxBytes int64) (*Manifest, error) {
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return nil, fmt.Errorf("not a gzip-compressed backup archive")
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	sums := map[string]FileInfo{}
	var manifest *Manifest
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("corrupt archive: %w", err)
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("unexpected entry type in archive: %s", hdr.Name)
		}
		name := path.Clean(hdr.Name)
		if name != hdr.Name || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || name == ".." {
			return nil, fmt.Errorf("unsafe path in archive: %q", hdr.Name)
		}
		if name != ManifestFile && name != DBFile && name != ExportFile && !strings.HasPrefix(name, MediaPrefix) {
			return nil, fmt.Errorf("unexpected file in archive: %q", name)
		}
		total += hdr.Size
		if total > maxBytes {
			return nil, fmt.Errorf("archive expands beyond the allowed size")
		}
		if name == ManifestFile {
			if hdr.Size > 16<<20 {
				return nil, fmt.Errorf("manifest too large")
			}
			b, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			manifest = &Manifest{}
			if err := json.Unmarshal(b, manifest); err != nil {
				return nil, fmt.Errorf("invalid manifest: %w", err)
			}
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return nil, fmt.Errorf("unsafe path in archive: %q", hdr.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
		if err != nil {
			return nil, fmt.Errorf("duplicate or invalid entry %q: %w", name, err)
		}
		h := sha256.New()
		n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(tr, hdr.Size))
		cerr := f.Close()
		if err != nil {
			return nil, err
		}
		if cerr != nil {
			return nil, cerr
		}
		sums[name] = FileInfo{Size: n, SHA256: hex.EncodeToString(h.Sum(nil))}
	}
	if manifest == nil {
		return nil, fmt.Errorf("archive has no manifest.json — is this a Family Holocron backup?")
	}
	if manifest.Format < 1 || manifest.Format > version.BackupFormat {
		return nil, fmt.Errorf("unsupported backup format %d", manifest.Format)
	}
	if _, ok := sums[DBFile]; !ok {
		return nil, fmt.Errorf("archive has no database")
	}
	for name, want := range manifest.Files {
		got, ok := sums[name]
		if !ok {
			return nil, fmt.Errorf("archive is incomplete: %s is missing", name)
		}
		if got != want {
			return nil, fmt.Errorf("checksum mismatch for %s — the archive is corrupt", name)
		}
	}
	for name := range sums {
		if _, ok := manifest.Files[name]; !ok {
			return nil, fmt.Errorf("archive contains unlisted file %s", name)
		}
	}
	return manifest, nil
}

func (s *Service) maxExtract(archiveSize int64) int64 {
	return max(1<<30, archiveSize*50)
}

// validateFile fully extracts and checks an archive in a scratch directory.
func (s *Service) validateFile(ctx context.Context, archivePath, scratch string) (*Manifest, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, _ := f.Stat()
	m, err := Extract(f, scratch, s.maxExtract(st.Size()))
	if err != nil {
		return nil, err
	}
	if _, err := s.Prepare(ctx, filepath.Join(scratch, DBFile)); err != nil {
		return nil, err
	}
	return m, nil
}

// ValidationError marks an archive that failed validation.
type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return "Backup validation failed: " + e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

// Upload stores an uploaded archive after fully validating it.
func (s *Service) Upload(ctx context.Context, actor store.Actor, r io.Reader) (*Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tmp, err := os.CreateTemp(s.BackupDir, ".upload-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	scratch, err := os.MkdirTemp(s.BackupDir, ".validate-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch)
	m, err := s.validateFile(ctx, tmp.Name(), scratch)
	if err != nil {
		return nil, &ValidationError{Err: err}
	}
	name := s.uniqueName("uploaded-")
	final := filepath.Join(s.BackupDir, name)
	if err := os.Rename(tmp.Name(), final); err != nil {
		return nil, err
	}
	m.Kind = KindUpload
	if err := writeJSONFile(final+".json", m); err != nil {
		return nil, err
	}
	_ = s.Store.RecordAudit(ctx, actor, store.AuditEntry{Action: store.ActionCreate, EntityType: store.EntitySystem,
		EntityLabel: name, Summary: "Uploaded backup " + name + " for restore"})
	return s.Get(name)
}

// ---- restore ------------------------------------------------------------------------

// RestoreResult reports what a restore did.
type RestoreResult struct {
	Restored       string      `json:"restored"`
	SafetySnapshot string      `json:"safetySnapshot"`
	Counts         store.Stats `json:"counts"`
	SessionKept    bool        `json:"sessionKept"`
}

// Restore replaces all data and media with the contents of a stored backup.
// A safety snapshot of the current state is taken first.
func (s *Service) Restore(ctx context.Context, actor store.Actor, name string, keep store.KeepSession) (*RestoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	archive, err := s.Path(name)
	if err != nil {
		return nil, err
	}
	staging, err := os.MkdirTemp(s.BackupDir, ".restore-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)
	if _, err := s.validateFile(ctx, archive, staging); err != nil {
		return nil, &ValidationError{Err: err}
	}

	safety, err := s.createStoredLocked(ctx, actor, KindSafety, "pre-restore-")
	if err != nil {
		return nil, fmt.Errorf("could not create the pre-restore safety snapshot, nothing was changed: %w", err)
	}

	if err := s.Store.RestoreFrom(ctx, filepath.Join(staging, DBFile), keep); err != nil {
		return nil, fmt.Errorf("restore database (current data left unchanged): %w", err)
	}
	if err := swapMedia(s.MediaDir, filepath.Join(staging, "media")); err != nil {
		s.logger().Error("media restore failed after database restore", "err", err, "safety", safety.Name)
		return nil, fmt.Errorf("the database was restored but media files could not be replaced (%v); restore %s to roll back", err, safety.Name)
	}
	stats, _ := s.Store.Stats(ctx)
	res := &RestoreResult{Restored: name, SafetySnapshot: safety.Name}
	if stats != nil {
		res.Counts = *stats
	}
	if keep.TokenHash != "" {
		if _, _, err := s.Store.GetSession(ctx, keep.TokenHash); err == nil {
			res.SessionKept = true
		}
	}
	auditActor := actor
	if !res.SessionKept {
		auditActor = store.Actor{Username: actor.Username} // user id may not exist in restored data
	}
	_ = s.Store.RecordAudit(ctx, auditActor, store.AuditEntry{Action: store.ActionRestore, EntityType: store.EntitySystem, EntityLabel: name,
		Summary: fmt.Sprintf("Restored backup %s (safety snapshot %s)", name, safety.Name)})
	if s.OnRestored != nil {
		s.OnRestored(ctx)
	}
	return res, nil
}

// swapMedia replaces the contents of mediaDir with those of newDir. mediaDir itself
// is kept (it may be a mount point). On failure it attempts to roll back.
func swapMedia(mediaDir, newDir string) error {
	if err := os.MkdirAll(mediaDir, 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(newDir, 0o750); err != nil {
		return err
	}
	stamp := time.Now().UnixNano()
	// Stage the incoming files on the media volume first: data and media may be
	// different mounts, and rename(2) cannot cross devices.
	local := filepath.Join(mediaDir, fmt.Sprintf(".restore-new-%d", stamp))
	if err := os.Rename(newDir, local); err != nil {
		if err := copyTree(newDir, local); err != nil {
			os.RemoveAll(local)
			return fmt.Errorf("stage media: %w", err)
		}
	}
	defer os.RemoveAll(local)
	newDir = local
	old := filepath.Join(mediaDir, fmt.Sprintf(".restore-old-%d", stamp))
	if err := os.Mkdir(old, 0o750); err != nil {
		return err
	}
	current, err := os.ReadDir(mediaDir)
	if err != nil {
		return err
	}
	var moved []string
	rollback := func() {
		for _, n := range moved {
			_ = os.Rename(filepath.Join(old, n), filepath.Join(mediaDir, n))
		}
	}
	for _, e := range current {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if err := os.Rename(filepath.Join(mediaDir, e.Name()), filepath.Join(old, e.Name())); err != nil {
			rollback()
			return err
		}
		moved = append(moved, e.Name())
	}
	incoming, err := os.ReadDir(newDir)
	if err != nil {
		rollback()
		return err
	}
	var placed []string
	for _, e := range incoming {
		if err := os.Rename(filepath.Join(newDir, e.Name()), filepath.Join(mediaDir, e.Name())); err != nil {
			for _, n := range placed {
				_ = os.RemoveAll(filepath.Join(mediaDir, n))
			}
			rollback()
			return err
		}
		placed = append(placed, e.Name())
	}
	return os.RemoveAll(old)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

func writeJSONFile(p string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o640)
}
