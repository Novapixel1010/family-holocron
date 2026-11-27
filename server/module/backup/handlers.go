package backup

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// ConfirmPhrase must be sent to confirm a restore.
const ConfirmPhrase = "RESTORE"

// Module exposes the backup service over HTTP (admin only).
type Module struct {
	Service        *Service
	MaxUploadBytes int64
}

// Register mounts routes.
func (m *Module) Register(rt *apiutil.Router) {
	rt.Handle("GET /api/v1/backups", apiutil.Admin, m.list)
	rt.Handle("POST /api/v1/backups", apiutil.Admin, m.create)
	rt.Handle("GET /api/v1/backups/download", apiutil.Admin, m.downloadFresh)
	rt.Handle("POST /api/v1/backups/upload", apiutil.Admin, m.upload)
	rt.Handle("GET /api/v1/backups/{name}/download", apiutil.Admin, m.download)
	rt.Handle("DELETE /api/v1/backups/{name}", apiutil.Admin, m.delete)
	rt.Handle("POST /api/v1/backups/{name}/restore", apiutil.Admin, m.restore)
}

func mapErr(err error) error {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return &apiutil.Error{Status: http.StatusUnprocessableEntity, Code: "invalid_backup", Message: ve.Error()}
	}
	if errors.Is(err, store.ErrNotFound) {
		return apiutil.NotFound("Backup")
	}
	return err
}

func (m *Module) list(w http.ResponseWriter, r *http.Request) error {
	items, err := m.Service.List()
	if err != nil {
		return err
	}
	return apiutil.OK(w, items)
}

func (m *Module) create(w http.ResponseWriter, r *http.Request) error {
	info, err := m.Service.CreateStored(r.Context(), apiutil.ActorFrom(r.Context()), KindManual, "holocron-backup-")
	if err != nil {
		return err
	}
	return apiutil.Created(w, info)
}

// downloadFresh creates a stored backup and streams it.
func (m *Module) downloadFresh(w http.ResponseWriter, r *http.Request) error {
	info, err := m.Service.CreateStored(r.Context(), apiutil.ActorFrom(r.Context()), KindManual, "holocron-backup-")
	if err != nil {
		return err
	}
	return m.serve(w, r, info.Name)
}

func (m *Module) serve(w http.ResponseWriter, r *http.Request, name string) error {
	p, err := m.Service.Path(name)
	if err != nil {
		return mapErr(err)
	}
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	w.Header().Set("X-Backup-Name", name)
	http.ServeContent(w, r, name, st.ModTime(), f)
	return nil
}

func (m *Module) download(w http.ResponseWriter, r *http.Request) error {
	return m.serve(w, r, r.PathValue("name"))
}

func (m *Module) delete(w http.ResponseWriter, r *http.Request) error {
	if err := m.Service.Delete(r.Context(), apiutil.ActorFrom(r.Context()), r.PathValue("name")); err != nil {
		return mapErr(err)
	}
	return apiutil.NoContent(w)
}

func (m *Module) upload(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, m.MaxUploadBytes)
	var body = r.Body
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		mr, err := r.MultipartReader()
		if err != nil {
			return apiutil.BadRequest("Invalid upload.")
		}
		for {
			part, err := mr.NextPart()
			if err != nil {
				return store.Invalid("file", "choose a backup archive to upload")
			}
			if part.FormName() == "file" {
				body = part
				break
			}
		}
	}
	info, err := m.Service.Upload(r.Context(), apiutil.ActorFrom(r.Context()), body)
	if err != nil {
		return mapErr(err)
	}
	return apiutil.Created(w, info)
}

func (m *Module) restore(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Confirm string `json:"confirm"`
	}
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	if in.Confirm != ConfirmPhrase {
		return &store.ValidationError{Message: `Type RESTORE to confirm. This replaces all current data.`, Fields: map[string]string{"confirm": "type RESTORE to confirm"}}
	}
	u := apiutil.UserFrom(r.Context())
	keep := store.KeepSession{TokenHash: apiutil.SessionFrom(r.Context()), UserID: u.ID, Username: u.Username}
	start := time.Now()
	res, err := m.Service.Restore(r.Context(), u.Actor(), r.PathValue("name"), keep)
	if err != nil {
		return mapErr(err)
	}
	return apiutil.OK(w, map[string]any{"result": res, "durationMs": time.Since(start).Milliseconds()})
}
