package server_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

// bearer is an HTTP client that authenticates only with a personal access token.
type bearer struct {
	e     *env
	token string
}

func (b *bearer) do(method, path string, body any) (int, json.RawMessage, http.Header) {
	b.e.t.Helper()
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, b.e.ts.URL+path, rd)
	req.Header.Set("Authorization", "Bearer "+b.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		b.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data, resp.Header
}

func (b *bearer) must(method, path string, body any, out any, want int) json.RawMessage {
	b.e.t.Helper()
	code, raw, _ := b.do(method, path, body)
	if code != want {
		b.e.t.Fatalf("bearer %s %s: status %d, want %d: %s", method, path, code, want, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			b.e.t.Fatal(err)
		}
	}
	return raw
}

func errorCode(raw json.RawMessage) string {
	var x struct {
		Error struct{ Code string } `json:"error"`
	}
	json.Unmarshal(raw, &x)
	return x.Error.Code
}

type accessToken struct {
	ID          int64      `json:"id"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"createdAt"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	LastUsedAt  *time.Time `json:"lastUsedAt"`
	Token       string     `json:"token"`
}

var tokenFormat = regexp.MustCompile(`^fh_pat_[0-9A-Za-z]{32}$`)

func TestPersonalAccessTokens(t *testing.T) {
	e := newEnv(t)
	admin := setupAdmin(t, e)

	// Validation.
	var verr json.RawMessage
	if code := admin.do("POST", "/api/v1/user/access-tokens", map[string]any{"description": "  "}, &verr); code != 422 || !strings.Contains(string(verr), "description") {
		t.Fatalf("blank description: %d %s", code, verr)
	}
	for _, days := range []int{0, -1, 3651} {
		if code := admin.do("POST", "/api/v1/user/access-tokens", map[string]any{"description": "x", "expiresInDays": days}, &verr); code != 422 {
			t.Fatalf("expiresInDays %d: %d %s", days, code, verr)
		}
	}

	// Create: the plaintext comes back once; the hash never does.
	var raw json.RawMessage
	admin.must("POST", "/api/v1/user/access-tokens", map[string]any{"description": "GEDCOM Sync", "expiresInDays": nil}, &raw, 201)
	var created accessToken
	json.Unmarshal(raw, &created)
	if !tokenFormat.MatchString(created.Token) || created.Description != "GEDCOM Sync" || created.ExpiresAt != nil || created.LastUsedAt != nil {
		t.Fatalf("created token: %s", raw)
	}
	if strings.Contains(string(raw), "tokenHash") || strings.Contains(string(raw), "token_hash") {
		t.Fatalf("hash leaked: %s", raw)
	}
	var withExpiry accessToken
	admin.must("POST", "/api/v1/user/access-tokens", map[string]any{"description": "Home Assistant", "expiresInDays": 30}, &withExpiry, 201)
	if withExpiry.ExpiresAt == nil || withExpiry.ExpiresAt.Sub(time.Now().Add(30*24*time.Hour)).Abs() > time.Minute {
		t.Fatalf("30-day expiry: %+v", withExpiry.ExpiresAt)
	}
	if withExpiry.Token == created.Token {
		t.Fatal("tokens must be unique")
	}

	admin.must("GET", "/api/v1/user/access-tokens", nil, &raw, 200)
	if strings.Contains(string(raw), "fh_pat_") || strings.Contains(string(raw), "okenHash") {
		t.Fatalf("list must not include secrets: %s", raw)
	}
	var list []accessToken
	json.Unmarshal(raw, &list)
	if len(list) != 2 || list[0].Description != "Home Assistant" || list[1].Description != "GEDCOM Sync" {
		t.Fatalf("list (newest first): %s", raw)
	}

	// The token acts as its owner: identity, role and attribution.
	pat := &bearer{e: e, token: created.Token}
	var me struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	pat.must("GET", "/api/v1/auth/me", nil, &me, 200)
	if me.Username != "root" || me.Role != "admin" {
		t.Fatalf("token identity: %+v", me)
	}
	var p person
	pat.must("POST", "/api/v1/persons", map[string]any{"givenName": "Via", "surname": "Token"}, &p, 201)
	if p.CreatedBy == nil || p.CreatedBy.Username != "root" {
		t.Fatalf("attribution via token: %+v", p.CreatedBy)
	}
	var audit struct {
		Items []struct {
			Username string `json:"username"`
			Summary  string `json:"summary"`
		} `json:"items"`
	}
	admin.must("GET", fmt.Sprintf("/api/v1/audit?personId=%d", p.ID), nil, &audit, 200)
	if len(audit.Items) != 1 || audit.Items[0].Username != "root" {
		t.Fatalf("audit via token: %+v", audit.Items)
	}
	pat.must("GET", "/api/v1/backups", nil, nil, 200) // admin-only route works for an admin's token

	// Use is recorded.
	admin.must("GET", "/api/v1/user/access-tokens", nil, &list, 200)
	if list[1].LastUsedAt == nil || time.Since(*list[1].LastUsedAt) > time.Minute || list[0].LastUsedAt != nil {
		t.Fatalf("lastUsedAt: %+v / %+v", list[1].LastUsedAt, list[0].LastUsedAt)
	}

	// A token can list and revoke tokens but cannot mint new ones.
	pat.must("GET", "/api/v1/user/access-tokens", nil, nil, 200)
	if code, body, _ := pat.do("POST", "/api/v1/user/access-tokens", map[string]any{"description": "child"}); code != 403 || errorCode(body) != "session_required" {
		t.Fatalf("token minting a token: %d %s", code, body)
	}

	// Bad tokens are rejected outright, even on public routes, and a bearer header wins over a cookie.
	for _, bad := range []string{"", "nonsense", "fh_pat_short", "memos_pat_" + strings.Repeat("a", 32), "fh_pat_" + strings.Repeat("a", 32), created.Token + "x"} {
		b := &bearer{e: e, token: bad}
		code, body, hdr := b.do("GET", "/api/v1/status", nil)
		if code != 401 || errorCode(body) != "invalid_token" || !strings.HasPrefix(hdr.Get("WWW-Authenticate"), "Bearer") {
			t.Fatalf("bad token %q: %d %s %v", bad, code, body, hdr)
		}
	}
	req, _ := http.NewRequest("GET", e.ts.URL+"/api/v1/persons", nil)
	req.Header.Set("Authorization", "Bearer fh_pat_"+strings.Repeat("Z", 32))
	if resp, _ := admin.c.Do(req); resp.StatusCode != 401 {
		t.Fatalf("invalid bearer with a valid cookie: %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	// Other schemes are ignored (cookie still applies).
	req, _ = http.NewRequest("GET", e.ts.URL+"/api/v1/persons", nil)
	req.SetBasicAuth("root", "correct-horse")
	if resp, _ := admin.c.Do(req); resp.StatusCode != 200 {
		t.Fatalf("basic auth header with cookie: %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	// Tokens inherit the owner's role and are scoped to the owner.
	admin.must("POST", "/api/v1/users", map[string]string{"username": "vera", "password": "viewer-pass", "role": "viewer"}, nil, 201)
	viewer := e.client()
	viewer.must("POST", "/api/v1/auth/signin", map[string]string{"username": "vera", "password": "viewer-pass"}, nil, 200)
	var vtok accessToken
	viewer.must("POST", "/api/v1/user/access-tokens", map[string]any{"description": "Reader", "expiresInDays": 7}, &vtok, 201)
	vpat := &bearer{e: e, token: vtok.Token}
	vpat.must("GET", "/api/v1/persons", nil, nil, 200)
	vpat.must("POST", "/api/v1/persons", map[string]any{"givenName": "Nope"}, nil, 403)
	vpat.must("GET", "/api/v1/backups", nil, nil, 403)
	viewer.must("GET", "/api/v1/user/access-tokens", nil, &list, 200)
	if len(list) != 1 || list[0].Description != "Reader" {
		t.Fatalf("viewer sees only their tokens: %+v", list)
	}
	viewer.must("DELETE", fmt.Sprintf("/api/v1/user/access-tokens/%d", created.ID), nil, nil, 404)
	vpat.must("DELETE", fmt.Sprintf("/api/v1/user/access-tokens/%d", created.ID), nil, nil, 404)
	pat.must("GET", "/api/v1/auth/me", nil, nil, 200) // still valid

	// A temporary password locks the token like the session.
	var vera struct {
		ID int64 `json:"id"`
	}
	var users []struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	}
	admin.must("GET", "/api/v1/users", nil, &users, 200)
	for _, u := range users {
		if u.Username == "vera" {
			vera.ID = u.ID
		}
	}
	admin.must("PATCH", fmt.Sprintf("/api/v1/users/%d", vera.ID), map[string]any{"password": "temporary-9", "mustChangePassword": true}, nil, 200)
	if code, body, _ := vpat.do("GET", "/api/v1/persons", nil); code != 403 || errorCode(body) != "password_change_required" {
		t.Fatalf("token while password change required: %d %s", code, body)
	}
	vpat.must("GET", "/api/v1/auth/me", nil, nil, 200)
	vpat.must("POST", "/api/v1/auth/change-password", map[string]string{"currentPassword": "temporary-9", "newPassword": "viewer-pass-2"}, nil, 200)
	vpat.must("GET", "/api/v1/persons", nil, nil, 200)

	// Disabled owners can't use their tokens; re-enabling restores them.
	admin.must("PATCH", fmt.Sprintf("/api/v1/users/%d", vera.ID), map[string]any{"status": "disabled"}, nil, 200)
	if code, body, _ := vpat.do("GET", "/api/v1/persons", nil); code != 401 || errorCode(body) != "disabled" {
		t.Fatalf("disabled owner: %d %s", code, body)
	}
	admin.must("PATCH", fmt.Sprintf("/api/v1/users/%d", vera.ID), map[string]any{"status": "active"}, nil, 200)
	vpat.must("GET", "/api/v1/persons", nil, nil, 200)

	// Expired tokens are refused.
	db, err := sql.Open("sqlite", "file:"+e.prof.DBPath()+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE user_access_tokens SET expires_at = ? WHERE id = ?`, time.Now().Add(-time.Second).Unix(), vtok.ID); err != nil {
		t.Fatal(err)
	}
	if code, body, _ := vpat.do("GET", "/api/v1/persons", nil); code != 401 || errorCode(body) != "token_expired" {
		t.Fatalf("expired token: %d %s", code, body)
	}
	var stored string
	db.QueryRow(`SELECT token_hash FROM user_access_tokens WHERE id = ?`, created.ID).Scan(&stored)
	if len(stored) != 64 || strings.Contains(stored, "fh_pat_") {
		t.Fatalf("stored hash: %q", stored)
	}

	// Backups include tokens: revoke after a backup, restore, and the token works again.
	code, archive, _ := admin.raw("/api/v1/backups/download")
	if code != 200 {
		t.Fatalf("download backup: %d", code)
	}
	admin.must("DELETE", fmt.Sprintf("/api/v1/user/access-tokens/%d", created.ID), nil, nil, 204)
	if code, body, _ := pat.do("GET", "/api/v1/auth/me", nil); code != 401 || errorCode(body) != "invalid_token" {
		t.Fatalf("revoked token: %d %s", code, body)
	}
	admin.must("DELETE", fmt.Sprintf("/api/v1/user/access-tokens/%d", created.ID), nil, nil, 404)
	var info struct {
		Name string `json:"name"`
	}
	if code := admin.upload("/api/v1/backups/upload", "file", "pat.tar.gz", archive, nil, &info); code != 201 {
		t.Fatalf("upload backup: %d", code)
	}
	admin.must("POST", "/api/v1/backups/"+info.Name+"/restore", map[string]string{"confirm": "RESTORE"}, nil, 200)
	pat.must("GET", "/api/v1/auth/me", nil, nil, 200)

	// Token creation and revocation are audited against the owner.
	admin.must("GET", "/api/v1/audit?entityType=user", nil, &audit, 200)
	var summaries []string
	for _, a := range audit.Items {
		summaries = append(summaries, a.Username+": "+a.Summary)
	}
	all := strings.Join(summaries, "\n")
	for _, want := range []string{`root: Created access token "GEDCOM Sync"`, `vera: Created access token "Reader"`} {
		if !strings.Contains(all, want) {
			t.Fatalf("audit missing %q in:\n%s", want, all)
		}
	}
	// A revoke is logged; after the restore the pre-backup log is back, so check a fresh one.
	admin.must("DELETE", fmt.Sprintf("/api/v1/user/access-tokens/%d", withExpiry.ID), nil, nil, 204)
	admin.must("GET", "/api/v1/audit?entityType=user", nil, &audit, 200)
	if audit.Items[0].Summary != `Revoked access token "Home Assistant"` {
		t.Fatalf("revoke audit: %+v", audit.Items[0])
	}
}
