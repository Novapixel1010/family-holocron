// Package media implements file uploads, metadata and authenticated file serving.
package media

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// Module is the media domain module.
type Module struct {
	Store          *store.Store
	Dir            string
	MaxUploadBytes int64
	Logger         *slog.Logger
}

// Register mounts routes.
func (m *Module) Register(rt *apiutil.Router) {
	rt.Handle("GET /api/v1/media", apiutil.Viewer, m.list)
	rt.Handle("POST /api/v1/media", apiutil.Editor, m.upload)
	rt.Handle("GET /api/v1/media/{id}", apiutil.Viewer, m.get)
	rt.Handle("PUT /api/v1/media/{id}", apiutil.Editor, m.update)
	rt.Handle("DELETE /api/v1/media/{id}", apiutil.Editor, m.delete)
	rt.Handle("GET /api/v1/media/{id}/file", apiutil.Viewer, m.file)
}

// allowed lists accepted MIME types (sniffed from content) and their extension.
var allowed = map[string]string{
	"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp", "image/bmp": ".bmp",
	"application/pdf": ".pdf", "text/plain; charset=utf-8": ".txt",
	"audio/mpeg": ".mp3", "audio/wave": ".wav", "audio/ogg": ".ogg", "video/mp4": ".mp4", "video/webm": ".webm",
	"application/ogg": ".ogg",
}

// inline lists types safe to display in the browser.
func inline(mt string) bool {
	return strings.HasPrefix(mt, "image/") || strings.HasPrefix(mt, "audio/") || strings.HasPrefix(mt, "video/") || mt == "application/pdf"
}

func (m *Module) list(w http.ResponseWriter, r *http.Request) error {
	pid, err := apiutil.QueryID(r, "personId")
	if err != nil {
		return err
	}
	items, total, err := m.Store.ListMedia(r.Context(), store.FindMedia{
		PersonID: pid, Query: r.URL.Query().Get("q"),
		Limit: apiutil.QueryInt(r, "limit", 60, 1, 500), Offset: apiutil.QueryInt(r, "offset", 0, 0, 1<<30),
	})
	if err != nil {
		return err
	}
	return apiutil.OK(w, apiutil.Page[*store.Media]{Items: items, Total: total})
}

func (m *Module) upload(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, m.MaxUploadBytes+1<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return err
		}
		return apiutil.BadRequest("Expected a multipart form upload.")
	}
	defer r.MultipartForm.RemoveAll()
	f, hdr, err := r.FormFile("file")
	if err != nil {
		return store.Invalid("file", "choose a file to upload")
	}
	defer f.Close()
	if hdr.Size > m.MaxUploadBytes {
		return apiutil.Errorf(http.StatusRequestEntityTooLarge, "too_large", "The file is larger than the %d MiB limit.", m.MaxUploadBytes>>20)
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	mt := http.DetectContentType(head[:n])
	ext, ok := allowed[mt]
	if !ok {
		return store.Invalid("file", fmt.Sprintf("files of type %s are not supported (images, PDF, plain text, audio and video are)", mt))
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	uidBytes := make([]byte, 12)
	if _, err := rand.Read(uidBytes); err != nil {
		return err
	}
	uid := hex.EncodeToString(uidBytes)
	rel := filepath.Join(uid[:2], uid+ext)
	abs := filepath.Join(m.Dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(abs), ".upload-*")
	if err != nil {
		return err
	}
	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, h), f)
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), abs)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return err
	}
	var personIDs []int64
	for _, s := range r.MultipartForm.Value["personIds"] {
		for _, part := range strings.Split(s, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil && id > 0 {
				personIDs = append(personIDs, id)
			}
		}
	}
	title := r.FormValue("title")
	if title == "" {
		title = strings.TrimSuffix(hdr.Filename, filepath.Ext(hdr.Filename))
	}
	item, err := m.Store.CreateMedia(r.Context(), apiutil.ActorFrom(r.Context()), &store.Media{
		UID: uid, StoragePath: filepath.ToSlash(rel), OriginalName: filepath.Base(hdr.Filename), MimeType: mt,
		SizeBytes: size, SHA256: hex.EncodeToString(h.Sum(nil)), Title: title,
		Description: r.FormValue("description"), TakenDate: r.FormValue("takenDate"), PersonIDs: personIDs,
	})
	if err != nil {
		os.Remove(abs)
		return err
	}
	return apiutil.Created(w, item)
}

func (m *Module) get(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	item, err := m.Store.GetMedia(r.Context(), id)
	if err == store.ErrNotFound {
		return apiutil.NotFound("Media item")
	}
	if err != nil {
		return err
	}
	return apiutil.OK(w, item)
}

func (m *Module) update(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	var in store.MediaInput
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	item, err := m.Store.UpdateMedia(r.Context(), apiutil.ActorFrom(r.Context()), id, in)
	if err == store.ErrNotFound {
		return apiutil.NotFound("Media item")
	}
	if err != nil {
		return err
	}
	return apiutil.OK(w, item)
}

func (m *Module) delete(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	item, err := m.Store.GetMedia(r.Context(), id)
	if err == store.ErrNotFound {
		return apiutil.NotFound("Media item")
	}
	if err != nil {
		return err
	}
	if err := m.Store.DeleteMedia(r.Context(), apiutil.ActorFrom(r.Context()), id); err != nil {
		return err
	}
	if p, err := m.safePath(item.StoragePath); err == nil {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) && m.Logger != nil {
			m.Logger.Warn("could not remove media file", "path", p, "err", err)
		}
	}
	return apiutil.NoContent(w)
}

func (m *Module) safePath(rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", errors.New("invalid media path")
	}
	return filepath.Join(m.Dir, clean), nil
}

func (m *Module) file(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	item, err := m.Store.GetMedia(r.Context(), id)
	if err == store.ErrNotFound {
		return apiutil.NotFound("Media item")
	}
	if err != nil {
		return err
	}
	p, err := m.safePath(item.StoragePath)
	if err != nil {
		return err
	}
	f, err := os.Open(p)
	if os.IsNotExist(err) {
		return apiutil.Errorf(http.StatusNotFound, "file_missing", "The file for this media item is missing from storage.")
	}
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	disp := "attachment"
	if inline(item.MimeType) && r.URL.Query().Get("download") == "" {
		disp = "inline"
	}
	w.Header().Set("Content-Type", item.MimeType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disp, map[string]string{"filename": item.OriginalName}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	csp := "default-src 'none'; img-src 'self'; media-src 'self'; style-src 'unsafe-inline'"
	if item.MimeType != "application/pdf" { // browsers' PDF viewers refuse to run sandboxed
		csp += "; sandbox"
	}
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("ETag", `"`+item.SHA256+`"`)
	http.ServeContent(w, r, "", st.ModTime(), f)
	return nil
}
