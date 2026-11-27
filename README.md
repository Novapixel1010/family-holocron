# Family Holocron

A self-hosted, modular family tree for your family's history. It ships as a single Go binary with an embedded React UI and a SQLite database, and `docker compose up -d` starts it.

- **Interactive tree.** Ancestor and descendant views with pan and zoom, expand and collapse, click-to-inspect, and refocusing on any person.
- **Timeline.** Births, deaths, marriages, residences, custom life events, family milestones and historical events in order. Filter by person, type, text and year range.
- **Person profiles.** Structured vital facts plus a Markdown **About** biography. There are also notes, media, family links and a per-person change history.
- **Attribution everywhere.** Every person, relationship, event, note and media item records who created it and who last changed it. Each change also writes an audit-log entry with a field-by-field diff, and the UI shows both.
- **Backup and restore.** One archive holds the database, media and a portable JSON export. Restore needs explicit confirmation. The archive is checked against checksums, an integrity check and the schema version, and a safety snapshot of the current data is taken before anything changes.
- **No lock-in.** GEDCOM 5.5.1 import and export, plus a full JSON export.
- **Plugins.** Plugins can add their own tables, API routes, nav pages and profile tabs without touching core tables. The planned `family-holocron-dna` plugin is meant to use this.

## Quick start (Docker Compose)

```bash
cp .env.example .env    # optional
docker compose up -d
```

Open <http://localhost:8080>. On the first visit you create the administrator account. After that you can invite relatives from **Settings → Users** as viewers, editors or admins.

Data lives in two named volumes:

| Volume | Mount | Contents |
|---|---|---|
| `holocron-data` | `/var/opt/holocron/data` | `holocron.db` (SQLite, WAL) and `backups/` |
| `holocron-media` | `/var/opt/holocron/media` | uploaded photos and documents |

The container runs as an unprivileged user (uid 10001) and has a built-in healthcheck (`holocron healthcheck`). If you put it behind HTTPS, set `HOLOCRON_SECURE_COOKIES=true`.

## Configuration

| Env var | Flag | Default |
|---|---|---|
| `HOLOCRON_PORT` / `HOLOCRON_ADDR` | `--port` / `--addr` | `8080` |
| `HOLOCRON_DATA_DIR` | `--data` | `./data` |
| `HOLOCRON_MEDIA_DIR` | `--media` | `<data>/media` |
| `HOLOCRON_BACKUP_DIR` | `--backups` | `<data>/backups` |
| `HOLOCRON_MAX_UPLOAD_MB` | `--max-upload-mb` | `64` |
| `HOLOCRON_SESSION_TTL_HOURS` | `--session-ttl-hours` | `720` |
| `HOLOCRON_SECURE_COOKIES` | `--secure-cookies` | `false` |
| `HOLOCRON_DISABLED_PLUGINS` | | (none) |
| `HOLOCRON_DEV` | `--dev` | `false` |

## Development

Requirements: Go 1.26+ and Node 22+.

```bash
cd web && npm install && npm run build && cd ..   # builds into server/frontend/dist
go run ./bin/holocron --data ./data                # http://localhost:8080
```

For frontend hot reload, run the Go server and then start Vite with `cd web && npm run dev` (port 5173). Vite proxies `/api` to the Go server.

Tests:

```bash
go test -race ./...
```

```bash
cd web && npm run typecheck && npm test
```

The Go tests cover the SQLite driver (WAL and foreign keys, persistence across reopen, attribution and audit for every entity, cycle prevention, concurrent writes, snapshot and restore), the full HTTP API (setup, roles, attribution, tree, timeline, media, backup, tampered archives, restore, rollback to the safety snapshot, GEDCOM round trip, plugin gating), archive path-traversal guards, and GEDCOM and date parsing.

## Architecture

The repository layout, SQLite schema, API routes, UI slots and plugin interface are described in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## License

Apache 2.0. See [LICENSE](LICENSE).
