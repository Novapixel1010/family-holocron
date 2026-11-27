// Package profile resolves runtime configuration from flags and HOLOCRON_* environment variables.
package profile

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Profile is the runtime configuration of a Family Holocron instance.
type Profile struct {
	Addr           string        // listen address, e.g. ":8080"
	DataDir        string        // holds holocron.db
	MediaDir       string        // uploaded media files
	BackupDir      string        // stored backup archives
	MaxUploadBytes int64         // per-request upload limit
	SessionTTL     time.Duration // session lifetime
	SecureCookies  bool          // set Secure on cookies (behind HTTPS)
	Dev            bool          // development mode (verbose logging)
	DisabledPlugin []string      // plugin ids disabled via env (comma separated)
}

// DBPath returns the SQLite database file path.
func (p *Profile) DBPath() string { return filepath.Join(p.DataDir, "holocron.db") }

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// Load parses flags (from args) layered over environment variables.
func Load(args []string) (*Profile, error) {
	fs := flag.NewFlagSet("holocron", flag.ContinueOnError)
	p := &Profile{}
	var maxUploadMB int64
	var sessionHours int
	var port string
	fs.StringVar(&p.Addr, "addr", env("HOLOCRON_ADDR", ""), "listen address (overrides --port)")
	fs.StringVar(&port, "port", env("HOLOCRON_PORT", "8080"), "listen port")
	fs.StringVar(&p.DataDir, "data", env("HOLOCRON_DATA_DIR", "./data"), "data directory (SQLite database)")
	fs.StringVar(&p.MediaDir, "media", env("HOLOCRON_MEDIA_DIR", ""), "media directory (default <data>/media)")
	fs.StringVar(&p.BackupDir, "backups", env("HOLOCRON_BACKUP_DIR", ""), "backup directory (default <data>/backups)")
	mu, _ := strconv.ParseInt(env("HOLOCRON_MAX_UPLOAD_MB", "64"), 10, 64)
	fs.Int64Var(&maxUploadMB, "max-upload-mb", mu, "maximum upload size in MiB")
	sh, _ := strconv.Atoi(env("HOLOCRON_SESSION_TTL_HOURS", "720"))
	fs.IntVar(&sessionHours, "session-ttl-hours", sh, "session lifetime in hours")
	fs.BoolVar(&p.SecureCookies, "secure-cookies", envBool("HOLOCRON_SECURE_COOKIES", false), "mark cookies Secure (use behind HTTPS)")
	fs.BoolVar(&p.Dev, "dev", envBool("HOLOCRON_DEV", false), "development mode")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if p.Addr == "" {
		p.Addr = ":" + port
	}
	if p.MediaDir == "" {
		p.MediaDir = filepath.Join(p.DataDir, "media")
	}
	if p.BackupDir == "" {
		p.BackupDir = filepath.Join(p.DataDir, "backups")
	}
	if maxUploadMB <= 0 {
		return nil, errors.New("max-upload-mb must be positive")
	}
	if sessionHours <= 0 {
		return nil, errors.New("session-ttl-hours must be positive")
	}
	p.MaxUploadBytes = maxUploadMB << 20
	p.SessionTTL = time.Duration(sessionHours) * time.Hour
	if v := env("HOLOCRON_DISABLED_PLUGINS", ""); v != "" {
		p.DisabledPlugin = splitComma(v)
	}
	return p, nil
}

// Prepare creates the directories the profile refers to.
func (p *Profile) Prepare() error {
	for _, d := range []string{p.DataDir, p.MediaDir, p.BackupDir} {
		abs, err := filepath.Abs(d)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(abs, 0o750); err != nil {
			return fmt.Errorf("create %s: %w", abs, err)
		}
	}
	return nil
}

func splitComma(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
