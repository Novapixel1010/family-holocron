// Package server wires the domain modules, plugins and embedded frontend into an http.Server.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/novapixel1010/family-holocron/internal/profile"
	"github.com/novapixel1010/family-holocron/plugin"
	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/server/frontend"
	"github.com/novapixel1010/family-holocron/server/module/audit"
	"github.com/novapixel1010/family-holocron/server/module/auth"
	"github.com/novapixel1010/family-holocron/server/module/backup"
	"github.com/novapixel1010/family-holocron/server/module/exchange"
	"github.com/novapixel1010/family-holocron/server/module/media"
	"github.com/novapixel1010/family-holocron/server/module/person"
	"github.com/novapixel1010/family-holocron/server/module/system"
	"github.com/novapixel1010/family-holocron/server/module/timeline"
	"github.com/novapixel1010/family-holocron/server/module/tree"
	"github.com/novapixel1010/family-holocron/store"
)

// Options configures a Server.
type Options struct {
	Profile       *profile.Profile
	Store         *store.Store
	Plugins       *plugin.Registry
	Logger        *slog.Logger
	SchemaVersion func() int
	Prepare       backup.PrepareFunc
}

// Server is the HTTP server.
type Server struct {
	Handler http.Handler
	Auth    *auth.Module
	Backup  *backup.Service
	opts    Options
}

// New builds the server and mounts every module.
func New(ctx context.Context, o Options) (*Server, error) {
	if err := o.Plugins.Init(ctx, plugin.Host{Store: o.Store, Logger: o.Logger}); err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	rt := &apiutil.Router{Mux: mux, Logger: o.Logger}
	p := o.Profile

	authMod := auth.New(o.Store, o.Logger, p.SessionTTL, p.SecureCookies)
	backupSvc := &backup.Service{
		Store: o.Store, MediaDir: p.MediaDir, BackupDir: p.BackupDir, SchemaVersion: o.SchemaVersion,
		Prepare: o.Prepare, Logger: o.Logger,
		OnRestored: func(ctx context.Context) {
			if err := o.Plugins.Reload(ctx); err != nil {
				o.Logger.Error("reload plugins after restore", "err", err)
			}
		},
	}
	modules := []interface{ Register(*apiutil.Router) }{
		&system.Module{Store: o.Store, Plugins: o.Plugins, SchemaVersion: o.SchemaVersion, Started: time.Now()},
		authMod,
		&person.Module{Store: o.Store},
		&timeline.Module{Store: o.Store},
		&tree.Module{Store: o.Store},
		&media.Module{Store: o.Store, Dir: p.MediaDir, MaxUploadBytes: p.MaxUploadBytes, Logger: o.Logger},
		&backup.Module{Service: backupSvc, MaxUploadBytes: max(p.MaxUploadBytes, 4<<30)},
		&exchange.Module{Store: o.Store, MaxUploadBytes: p.MaxUploadBytes},
		&audit.Module{Store: o.Store},
	}
	for _, m := range modules {
		m.Register(rt)
	}
	o.Plugins.MountRoutes(rt)
	mux.Handle("/", frontend.Handler())

	var h http.Handler = mux
	h = authMod.Middleware(h)
	h = sameOrigin(h)
	h = securityHeaders(h)
	h = logRequests(o.Logger, h)
	return &Server{Handler: h, Auth: authMod, Backup: backupSvc, opts: o}, nil
}

// sameOrigin rejects state-changing requests whose Origin does not match the host
// (defence in depth on top of SameSite=Lax cookies).
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if origin := r.Header.Get("Origin"); origin != "" && origin != "null" {
				u, err := url.Parse(origin)
				if err != nil || !strings.EqualFold(u.Host, r.Host) {
					apiutil.WriteError(w, nil, apiutil.Errorf(http.StatusForbidden, "bad_origin", "Cross-origin request rejected."))
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "same-origin")
		if !strings.HasPrefix(r.URL.Path, "/api/v1/media/") {
			h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; media-src 'self'; frame-src 'self'; object-src 'self'")
		}
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		if strings.HasPrefix(r.URL.Path, "/api/") || sw.status >= 500 {
			logger.Debug("request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "dur", time.Since(start).String())
		}
	})
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }
