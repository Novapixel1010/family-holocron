// Package apiutil provides the HTTP plumbing shared by all server modules:
// a router with per-route access levels, JSON helpers and error mapping.
package apiutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/novapixel1010/family-holocron/store"
)

// Access is the minimum permission a route requires.
type Access int

const (
	Public Access = iota
	Viewer
	Editor
	Admin
)

func (a Access) allows(u *store.User) bool {
	switch a {
	case Public:
		return true
	case Viewer:
		return u != nil && u.Role.Rank() >= store.RoleViewer.Rank()
	case Editor:
		return u != nil && u.Role.Rank() >= store.RoleEditor.Rank()
	case Admin:
		return u != nil && u.Role == store.RoleAdmin
	}
	return false
}

// HandlerFunc is an http handler that may return an error to be rendered as JSON.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// Error is an API error with an HTTP status.
type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e *Error) Error() string { return e.Message }

// Errorf builds an API error.
func Errorf(status int, code, format string, args ...any) *Error {
	return &Error{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

// BadRequest is a 400 error.
func BadRequest(format string, args ...any) *Error {
	return Errorf(http.StatusBadRequest, "bad_request", format, args...)
}

// NotFound is a 404 error.
func NotFound(what string) *Error {
	return Errorf(http.StatusNotFound, "not_found", "%s not found", what)
}

type ctxKey int

const (
	userKey ctxKey = iota
	sessionKey
	accessTokenKey
)

// WithUser stores the signed-in user (and session token hash) in ctx.
func WithUser(ctx context.Context, u *store.User, tokenHash string) context.Context {
	ctx = context.WithValue(ctx, userKey, u)
	return context.WithValue(ctx, sessionKey, tokenHash)
}

// UserFrom returns the signed-in user or nil.
func UserFrom(ctx context.Context) *store.User {
	u, _ := ctx.Value(userKey).(*store.User)
	return u
}

// SessionFrom returns the current session token hash or "".
func SessionFrom(ctx context.Context) string {
	s, _ := ctx.Value(sessionKey).(string)
	return s
}

// WithAccessToken marks the request as authenticated by personal access token id.
func WithAccessToken(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, accessTokenKey, id)
}

// AccessTokenFrom returns the id of the personal access token that authenticated
// the request, or 0 for cookie sessions and anonymous requests.
func AccessTokenFrom(ctx context.Context) int64 {
	id, _ := ctx.Value(accessTokenKey).(int64)
	return id
}

// ActorFrom returns the attribution actor for the request.
func ActorFrom(ctx context.Context) store.Actor {
	if u := UserFrom(ctx); u != nil {
		return u.Actor()
	}
	return store.SystemActor
}

// Router registers handlers on a ServeMux with access control.
type Router struct {
	Mux    *http.ServeMux
	Logger *slog.Logger
	// Gate, when set, may reject a request before the handler runs (e.g. disabled plugin).
	Gate func(r *http.Request) error
	// PasswordChangeAllowed lists the route patterns a user with a temporary password
	// may still call (everything else that needs a session is refused).
	PasswordChangeAllowed map[string]bool
}

// ErrPasswordChangeRequired is returned to users who must replace a temporary password.
var ErrPasswordChangeRequired = Errorf(http.StatusForbidden, "password_change_required",
	"Please change your temporary password before continuing.")

// Handle registers pattern (Go 1.22 mux syntax, e.g. "GET /api/v1/persons/{id}").
func (rt *Router) Handle(pattern string, access Access, h HandlerFunc) {
	gate := rt.Gate
	allowedDuringChange := rt.PasswordChangeAllowed[pattern]
	rt.Mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		u := UserFrom(r.Context())
		// A temporary password locks the session to the password-change routes.
		if u != nil && u.MustChangePassword && access != Public && !allowedDuringChange {
			WriteError(w, rt.Logger, ErrPasswordChangeRequired)
			return
		}
		if !access.allows(u) {
			if u == nil {
				WriteError(w, rt.Logger, Errorf(http.StatusUnauthorized, "unauthorized", "Please sign in to continue."))
			} else {
				WriteError(w, rt.Logger, Errorf(http.StatusForbidden, "forbidden", "You do not have permission to do that."))
			}
			return
		}
		if gate != nil {
			if err := gate(r); err != nil {
				WriteError(w, rt.Logger, err)
				return
			}
		}
		if err := h(w, r); err != nil {
			WriteError(w, rt.Logger, err)
		}
	})
}

// WithGate returns a copy of the router that applies gate to every route.
func (rt *Router) WithGate(gate func(r *http.Request) error) *Router {
	return &Router{Mux: rt.Mux, Logger: rt.Logger, Gate: gate, PasswordChangeAllowed: rt.PasswordChangeAllowed}
}

// WriteJSON writes v as JSON with status.
func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

// OK writes a 200 JSON response.
func OK(w http.ResponseWriter, v any) error { return WriteJSON(w, http.StatusOK, v) }

// Created writes a 201 JSON response.
func Created(w http.ResponseWriter, v any) error { return WriteJSON(w, http.StatusCreated, v) }

// NoContent writes a 204.
func NoContent(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// WriteError renders err as a JSON error envelope, mapping store errors.
func WriteError(w http.ResponseWriter, logger *slog.Logger, err error) {
	var apiErr *Error
	var ve *store.ValidationError
	var mbe *http.MaxBytesError
	switch {
	case errors.As(err, &apiErr):
	case errors.As(err, &ve):
		apiErr = &Error{Status: http.StatusUnprocessableEntity, Code: "validation", Message: ve.Error(), Fields: ve.Fields}
	case errors.Is(err, store.ErrNotFound):
		apiErr = NotFound("record")
	case errors.Is(err, store.ErrConflict):
		msg := strings.TrimPrefix(err.Error(), store.ErrConflict.Error()+": ")
		apiErr = &Error{Status: http.StatusConflict, Code: "conflict", Message: upperFirst(msg)}
	case errors.As(err, &mbe):
		apiErr = Errorf(http.StatusRequestEntityTooLarge, "too_large", "The upload is larger than the %d MiB limit.", mbe.Limit>>20)
	case errors.Is(err, context.Canceled):
		return
	default:
		if logger != nil {
			logger.Error("request failed", "err", err)
		}
		apiErr = Errorf(http.StatusInternalServerError, "internal", "Something went wrong on the server. Please try again.")
	}
	_ = WriteJSON(w, apiErr.Status, map[string]any{"error": apiErr})
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Decode reads a JSON body (max 4 MiB) into v.
func Decode(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 4<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return BadRequest("Invalid request body: %v", err)
	}
	return nil
}

// PathID parses the {name} path value as a positive int64.
func PathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, BadRequest("invalid %s", name)
	}
	return id, nil
}

// QueryInt parses an optional integer query parameter with bounds.
func QueryInt(r *http.Request, name string, def, min, max int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// QueryID parses an optional positive id query parameter.
func QueryID(r *http.Request, name string) (*int64, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return nil, BadRequest("invalid %s", name)
	}
	return &id, nil
}

// Page is a paginated list response.
type Page[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}
