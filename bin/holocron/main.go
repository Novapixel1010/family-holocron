// Command holocron runs the Family Holocron server.
//
//	holocron [serve] [flags]   start the server (default)
//	holocron healthcheck       probe the local /healthz endpoint (for Docker)
//	holocron version           print the version
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/novapixel1010/family-holocron/internal/profile"
	"github.com/novapixel1010/family-holocron/internal/version"
	"github.com/novapixel1010/family-holocron/plugin"
	"github.com/novapixel1010/family-holocron/plugin/builtin/stats"
	"github.com/novapixel1010/family-holocron/server"
	"github.com/novapixel1010/family-holocron/store"
	"github.com/novapixel1010/family-holocron/store/db/sqlite"
)

// registerPlugins is the single place where compiled-in plugins are listed.
// A future family-holocron-dna module would be added here.
func registerPlugins(r *plugin.Registry) {
	r.Register(stats.New())
}

func main() {
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve(args)
	case "healthcheck":
		err = healthcheck(args)
	case "version":
		fmt.Println(version.Version)
	default:
		err = fmt.Errorf("unknown command %q (want serve, healthcheck or version)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "holocron:", err)
		os.Exit(1)
	}
}

func serve(args []string) error {
	p, err := profile.Load(args)
	if err != nil {
		return err
	}
	level := slog.LevelInfo
	if p.Dev {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	if err := p.Prepare(); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sqlite.Open(ctx, p.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()
	st := store.New(db)
	reg := plugin.NewRegistry(p.DisabledPlugin...)
	registerPlugins(reg)

	srv, err := server.New(ctx, server.Options{
		Profile: p, Store: st, Plugins: reg, Logger: logger,
		SchemaVersion: sqlite.LatestSchemaVersion, Prepare: sqlite.PrepareRestoreSource,
	})
	if err != nil {
		return err
	}
	httpSrv := &http.Server{
		Addr: p.Addr, Handler: srv.Handler,
		ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("Family Holocron started", "version", version.Version, "addr", p.Addr, "data", p.DataDir, "media", p.MediaDir)
		errCh <- httpSrv.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
	return nil
}

func healthcheck(args []string) error {
	p, err := profile.Load(args)
	if err != nil {
		return err
	}
	addr := p.Addr
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: HTTP %d", resp.StatusCode)
	}
	return nil
}
