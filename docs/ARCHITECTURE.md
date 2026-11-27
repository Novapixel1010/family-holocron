# Family Holocron — Architecture

Family Holocron is a self-hosted family tree application shipped as a **single Go
binary** with an embedded React SPA and an embedded SQLite database. The layout
follows the structure popularised by `usememos/memos` (separate `server/`,
`store/` and `web/` trees, pure-Go SQLite, embedded Vite build, multi-stage
Docker image) but is scoped strictly to genealogy and to a plugin model that
lets specialised features (e.g. `family-holocron-dna`) live outside the core.

---

## 1. Repository layout

```
family-holocron/
├── bin/holocron/            # main(): `serve` (default), `healthcheck`, `version`
├── internal/
│   ├── profile/             # runtime configuration (flags + HOLOCRON_* env)
│   ├── version/             # build version, schema compatibility
│   ├── gendate/             # genealogical date parsing ("ABT 1850", "1850-03-12", …)
│   └── gedcom/              # GEDCOM 5.5.1 reader/writer (pure, no store deps)
├── store/                   # domain models + Driver interface (no SQL here)
│   ├── store.go             # Store = Driver + change dispatch to plugins
│   ├── *.go                 # User, Session, Person, Relationship, Event, Note, Media, Audit…
│   └── db/sqlite/           # the SQLite Driver (modernc.org/sqlite, WAL, FKs)
│       └── migration/       # embedded, versioned *.sql migrations
├── server/
│   ├── server.go            # wiring: modules, plugins, frontend, middleware
│   ├── apiutil/             # router w/ access levels, JSON + error helpers, auth ctx
│   ├── frontend/            # go:embed of web build output + SPA fallback
│   └── module/              # one package per domain
│       ├── auth/            # setup, login/logout, sessions, users, roles
│       ├── person/          # people, relationships, notes
│       ├── tree/            # ancestor/descendant tree assembly
│       ├── timeline/        # events CRUD + chronological timeline
│       ├── media/           # uploads, files, person links
│       ├── backup/          # archive create / validate / restore
│       ├── exchange/        # GEDCOM import + export
│       ├── audit/           # change log queries
│       └── system/          # health, status, settings, plugin manifests
├── plugin/
│   ├── plugin.go            # Plugin interfaces (manifest, routes, migrations, hooks)
│   ├── registry.go          # registry, enable/disable, route gating
│   └── builtin/stats/       # built-in example plugin (nav slot + API route)
├── web/                     # React + TS + Vite + Tailwind SPA
│   └── src/
│       ├── app/             # router, layout, providers
│       ├── api/             # typed API client + types
│       ├── components/ui/   # Button, Modal, Loading/Empty/Error states, …
│       ├── features/        # auth, people, person, tree, timeline, media, settings, home
│       └── plugins/         # web plugin registry + slots, core + stats plugins
├── Dockerfile               # node → go (CGO_ENABLED=0) → alpine runtime
├── compose.yaml             # docker compose up -d
└── .env.example
```

Dependency direction: `bin → server → module/* → store ← store/db/sqlite`.
Modules never import each other's internals; they share only `store` and
`apiutil`. Plugins see the `plugin.Host` (store, logger, change bus) only.

---

## 2. SQLite schema (migration `0001_init.sql`)

Conventions: `INTEGER PRIMARY KEY` ids, unix-second timestamps, every
user-editable genealogical record carries the **attribution quartet**
`created_by_user_id`, `updated_by_user_id`, `created_at`, `updated_at`
(user FKs are `ON DELETE SET NULL`, so history survives user removal).
Every mutation writes an `audit_log` row **in the same transaction**.

| Table | Purpose / key columns |
|---|---|
| `user` | `username` (unique, nocase), `display_name`, `email`, `password_hash` (bcrypt), `role` (`admin`/`editor`/`viewer`), `status` |
| `session` | `token_hash` (sha256 of cookie token), `user_id`, `expires_at`, `last_seen_at` |
| `person` | `uid` (stable external id), `given_name`, `surname`, `nickname`, `sex` (`M/F/X/U`), `birth_date`/`birth_sort`/`birth_place`, `death_date`/`death_sort`/`death_place`, `deceased`, **`about`** (Markdown biography), `portrait_media_id`, attribution |
| `relationship` | `type` (`parent`/`spouse`), `person1_id`, `person2_id` (parent→child; spouses normalised low→high), `subtype` (biological/adopted/step/foster · married/partner/divorced/separated), `start_date`, `end_date`, `notes`, attribution; unique `(type, person1_id, person2_id)` |
| `event` | `type` (marriage, residence, baptism, …, `custom`, `family`, `historical`), `title`, `date`/`sort_date`, `place`, `description` (Markdown), `relationship_id`, attribution |
| `event_participant` | `(event_id, person_id, role)` — many people per event |
| `note` | `person_id`, `title`, `body` (Markdown), attribution |
| `media` | `uid`, `storage_path`, `original_name`, `mime_type`, `size_bytes`, `sha256`, `title`, `description`, `taken_date`, attribution |
| `media_person` | `(media_id, person_id)` tags |
| `audit_log` | `user_id`, `username` snapshot, `action` (create/update/delete/import/restore/backup…), `entity_type`, `entity_id`, `entity_label`, `person_id` (for per-person history), `changes` (JSON field diff) |
| `system_setting` | key/value instance settings (e.g. `plugin.<id>.enabled`) |
| `plugin_kv` | `(plugin_id, key) → value` scratch storage for plugins |
| `plugin_migration` | per-plugin applied migration versions (plugins own their tables, prefixed `plg_<id>_`) |
| `schema_migration` | core migration versions |

Birth and death live on `person` (they are 1:1 vital facts and drive the tree
display); the timeline merges them in as synthetic `birth`/`death` items, so
the user sees a single chronological stream.

---

## 3. HTTP API (`/api/v1`, JSON, camelCase)

Access levels: **public**, **viewer** (any signed-in user), **editor**, **admin**.

| Method & path | Access | Notes |
|---|---|---|
| `GET /healthz` | public | liveness + DB ping (used by Docker healthcheck) |
| `GET /api/v1/status` | public | version, `setupRequired` |
| `POST /api/v1/auth/setup` | public | creates the first admin; 409 once any user exists |
| `POST /api/v1/auth/login` · `POST /auth/logout` · `GET /auth/me` | public/viewer | cookie session (HttpOnly, SameSite=Lax) |
| `PATCH /api/v1/auth/me` · `POST /auth/me/password` | viewer | own profile / password |
| `GET/POST /api/v1/users` · `PATCH /users/{id}` | admin | user management |
| `GET/POST /api/v1/persons` · `GET/PUT/DELETE /persons/{id}` | viewer/editor | `?q=&limit=&offset=&sort=` |
| `GET /api/v1/persons/{id}/family` | viewer | parents, spouses, children, siblings |
| `GET/POST /api/v1/relationships` · `PUT/DELETE /relationships/{id}` | viewer/editor | cycle prevention for parent links |
| `GET/POST /api/v1/persons/{id}/notes` · `PUT/DELETE /notes/{id}` | viewer/editor | |
| `GET/POST /api/v1/events` · `GET/PUT/DELETE /events/{id}` | viewer/editor | participants inline |
| `GET /api/v1/timeline` | viewer | `?personId=&types=&q=&from=&to=` |
| `GET /api/v1/tree/{id}` | viewer | `?generations=1..10&direction=ancestors\|descendants` |
| `GET/POST /api/v1/media` · `GET/PUT/DELETE /media/{id}` · `GET /media/{id}/file` | viewer/editor | multipart upload |
| `GET /api/v1/audit` | viewer | `?entityType=&entityId=&personId=&userId=&before=&limit=` |
| `GET /api/v1/stats` | viewer | dashboard counts |
| `GET /api/v1/backups` · `POST /backups` · `GET /backups/download` | admin | list stored / create stored / stream a fresh archive |
| `GET /backups/{name}/download` · `DELETE /backups/{name}` | admin | |
| `POST /api/v1/backups/upload` | admin | validates + stages an uploaded archive, returns its manifest |
| `POST /api/v1/backups/{name}/restore` | admin | body `{"confirm":"RESTORE"}`; takes a pre-restore safety snapshot first |
| `POST /api/v1/gedcom/import` · `GET /gedcom/export` | editor/viewer | GEDCOM 5.5.1 |
| `GET /api/v1/plugins` · `PUT /plugins/{id}` | viewer/admin | manifests + UI slots; enable/disable |
| `* /api/v1/plugins/{id}/…` | per route | plugin-owned routes, 404 when disabled |

Errors: `{"error":{"code":"not_found","message":"…","fields":{…}}}`.

### Backup archive format (`holocron-backup-YYYYMMDD-HHMMSS.tar.gz`)

```
manifest.json     # format=1, appVersion, schemaVersion, createdAt, createdBy, counts, sha256 of every file
holocron.db       # consistent snapshot via VACUUM INTO
export.json       # portable JSON of all genealogical records (no SQLite needed to read)
media/…           # every stored media file
```

Restore = validate (manifest, checksums, `PRAGMA integrity_check`, schema
version ≤ current, migrate a temporary copy) → write a **pre-restore safety
snapshot** to the backups directory → copy all tables from the attached copy in
one transaction (deferred FK checks + `foreign_key_check`) → swap media
directory contents → invalidate other sessions → audit `restore` entry.

---

## 4. Frontend routes & UI slots

| Route | Screen |
|---|---|
| `/setup`, `/login` | first-admin setup, sign-in |
| `/` | dashboard: counts, recent activity, quick actions |
| `/people`, `/people/new`, `/people/:id`, `/people/:id/edit` | directory and profile |
| `/tree`, `/tree/:id` | interactive ancestor/descendant tree (pan/zoom, expand/collapse, focus) |
| `/timeline` | chronological timeline with person/type/text filters |
| `/media` | media library |
| `/settings/:section` | account, users, backup & restore, import/export, activity, plugins |
| `/x/:pluginId/*` | plugin-contributed pages |

UI extension slots (`web/src/plugins/registry.ts`):

* `nav` — sidebar entries + routes under `/x/<plugin>`.
* `person.tab` — tabs on the person profile. The core About / Facts &
  Events / Family / Notes / Media / History tabs are themselves registered
  through this slot, so plugins are first-class peers.
* `settings.section` — extra settings sections.

A web plugin is shown only when the server reports the plugin as installed
and enabled (`GET /api/v1/plugins`).

## 5. Plugin hook interface (Go)

```go
type Plugin interface {
    Manifest() Manifest                     // id, name, version, UI slots
    Init(ctx context.Context, host Host) error
}
// Optional capabilities, discovered by type assertion:
type RouteProvider  interface { Routes() []Route }          // mounted under /api/v1/plugins/<id>/
type Migrator       interface { Migrations() []Migration }   // own tables: plg_<id>_*
type ChangeListener interface { OnChange(ctx context.Context, c store.Change) } // after commit
```

A DNA plugin would add `plg_dna_*` tables (kits, matches, segments), expose
`/api/v1/plugins/dna/...`, contribute a `person.tab` ("DNA") and a `nav`
entry ("Chromosome browser"), and react to `person` deletions via
`ChangeListener` — without touching any core table or view. Plugin tables
live in the same SQLite file, so backups and restores carry them automatically.
