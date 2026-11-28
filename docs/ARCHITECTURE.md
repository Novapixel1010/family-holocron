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
├── plugin/                  # core plugin API (interfaces + registry); no plugins live here
│   ├── plugin.go            # Plugin interfaces (manifest, routes, migrations, hooks)
│   ├── registry.go          # registry, enable/disable, route gating
├── plugins/                 # compile-time plugin registry: one folder per plugin
│   ├── plugins.go           # All(): the Go list of compiled-in plugins
│   ├── web.ts               # the matching list of plugin web UIs
│   ├── stats/               # Family statistics (Go + web/)
│   └── task/                # Research Tasks: Go, migrations/, web/
├── web/                     # React + TS + Vite + Tailwind SPA
│   └── src/
│       ├── app/             # router, layout, providers
│       ├── api/             # typed API client + types
│       ├── components/ui/   # Button, Modal, Loading/Empty/Error states, …
│       ├── features/        # auth, people, person, tree, timeline, media, settings, home
│       └── plugins/         # web slot registry + core profile tabs (plugin UIs live in /plugins)
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
| `user` | `username` (unique, nocase), `display_name`, `email`, `password_hash` (bcrypt; empty = no local password, reserved for identity-provider accounts), `must_change_password` (temporary password; a CHECK allows it only with a local password), `role` (`admin`/`editor`/`viewer`), `status` |
| `session` | `token_hash` (sha256 of cookie token), `user_id`, `expires_at`, `last_seen_at` |
| `person` | `uid` (stable external id), `given_name`, `surname`, `nickname`, `sex` (`M/F/X/U`), `birth_date`/`birth_sort`/`birth_place`, `death_date`/`death_sort`/`death_place`, `deceased`, **`about`** (Markdown biography), `portrait_media_id`, attribution |
| `relationship` | `type` (`parent`/`spouse`), `person1_id`, `person2_id` (parent→child; spouses normalised low→high), `subtype` (biological/adopted/step/foster · married/partner/divorced/separated), `start_date`, `end_date`, `notes`, attribution; unique `(type, person1_id, person2_id)` |
| `event` | `type` (marriage, residence, baptism, …, `custom`, `family`, `historical`), `title`, `date`/`sort_date`, `place`, `description` (Markdown), `relationship_id`, attribution |
| `event_participant` | `(event_id, person_id, role)` — many people per event |
| `note` | `person_id`, `title`, `body` (Markdown), attribution |
| `media` | `uid`, `storage_path`, `original_name`, `mime_type`, `size_bytes`, `sha256`, `title`, `description`, `taken_date`, attribution |
| `media_person` | `(media_id, person_id)` tags |
| `audit_log` | `user_id`, `username` snapshot, `action` (create/update/delete/import/restore/backup…), `entity_type`, `entity_id`, `entity_label`, `person_id` (for per-person history), `changes` (JSON field diff) |
| `bio_templates` | admin-managed Markdown templates for the About section: `name` (unique, nocase), `description`, `content_md`, `sort_order`, attribution. A starter "Standard Biography" is seeded once per database (marker `bio_templates.seeded`), so deleting every template sticks across restarts |
| `system_setting` | key/value instance settings (e.g. `plugin.<id>.enabled`) |
| `plugin_kv` | `(plugin_id, key) → value` scratch storage for plugins |
| `plugin_migration` | per-plugin applied migration versions (plugins own their tables, prefixed `plg_<id>_`) |
| `schema_migration` | core migration versions |

Birth and death live on `person` (they are 1:1 vital facts and drive the tree
display); the timeline merges them in as synthetic `birth`/`death` items, so
the user sees a single chronological stream.

**Date standard.** Every date a person enters — birth and death, event dates and
relationship start/end dates — is ISO `YYYY-MM-DD`, or `YYYY-MM` / `YYYY` when the day
or month is unknown, and every date is displayed that way (profile, tree, timeline,
family cards). The API (`POST`/`PUT /persons`, `/events`, `/relationships`) normalizes
padding (`1890-3-5` → `1890-03-05`) and rejects anything else, impossible calendar
dates, and future dates (except family milestones, which may be upcoming); a death
before the birth or a relationship ending before it starts is rejected, compared at
the precision both share (`1890-06` vs `1890` is fine). Values that predate the
standard (e.g. `ABT 1850` from a GEDCOM import) stay accepted while left unchanged, so
such records remain editable; the UI shows them gracefully (`c. 1850`, `before 1880`,
`12 MAR 1850` → `1850-03-12`) and offers to replace them. Forms use `DateField`:
typeable, clearable, with a calendar popover (portaled above modals, flips/clamps to
stay on screen, keyboard operable). GEDCOM import still accepts any GEDCOM date.
Media "date taken" keeps the flexible format.

---

## 3. HTTP API (`/api/v1`, JSON, camelCase)

Access levels: **public**, **viewer** (any signed-in user), **editor**, **admin**.

| Method & path | Access | Notes |
|---|---|---|
| `GET /healthz` | public | liveness + DB ping (used by Docker healthcheck) |
| `GET /api/v1/status` | public | version, `setupRequired` |
| `POST /api/v1/auth/setup` | public | creates the first admin; 409 once any user exists |
| `POST /api/v1/auth/signin` · `POST /auth/signout` · `GET /auth/me` | public/viewer | cookie session (HttpOnly, SameSite=Lax); sign in with username or email; responses include `mustChangePassword` |
| `POST /api/v1/auth/change-password` | viewer | checks the current password, length rules and that the new one differs; clears `mustChangePassword`; signs out other devices |
| `PATCH /api/v1/auth/me` | viewer | own display name / email |
| `GET /api/v1/identity-providers` · `GET /auth/oauth/callback` | public | **reserved** for OIDC/OAuth2: an empty list and `501` until identity providers exist, so adding SSO changes no existing URL |
| `GET/POST /api/v1/users` · `PATCH /users/{id}` | admin | user management |
| `GET/POST /api/v1/persons` · `GET/PUT/DELETE /persons/{id}` | viewer/editor | `?q=&limit=&offset=&sort=` |
| `GET /api/v1/persons/{id}/family` | viewer | parents, spouses, children, siblings |
| `GET/POST /api/v1/relationships` · `PUT/DELETE /relationships/{id}` | viewer/editor | cycle prevention for parent links |
| `GET/POST /api/v1/persons/{id}/notes` · `PUT/DELETE /notes/{id}` | viewer/editor | |
| `GET/POST /api/v1/events` · `GET/PUT/DELETE /events/{id}` | viewer/editor | participants inline |
| `GET /api/v1/timeline` | viewer | `?personId=&types=&q=&from=&to=` |
| `GET /api/v1/tree/{id}` | viewer | `?generations=1..12&direction=ancestors\|descendants`; returns a de-duplicated `people` + `links` graph (incl. couples, co-parents and partners) and a nested `root` |
| `GET/POST /api/v1/media` · `GET/PUT/DELETE /media/{id}` · `GET /media/{id}/file` | viewer/editor | multipart upload |
| `GET /api/v1/audit` | viewer | `?entityType=&entityId=&personId=&userId=&before=&limit=` |
| `GET /api/v1/bio-templates` · `POST /bio-templates` · `PUT/DELETE /bio-templates/{id}` | viewer / admin | biography templates (ordered by `sort_order`, `name`); the About editor's "Insert template" menu appends or replaces only on the user's explicit choice |
| `GET /api/v1/stats` | viewer | dashboard counts |
| `GET /api/v1/backups` · `POST /backups` · `GET /backups/download` | admin | list stored / create stored / stream a fresh archive |
| `GET /backups/{name}/download` · `DELETE /backups/{name}` | admin | |
| `POST /api/v1/backups/upload` | admin | validates + stages an uploaded archive, returns its manifest |
| `POST /api/v1/backups/{name}/restore` | admin | body `{"confirm":"RESTORE"}`; takes a pre-restore safety snapshot first |
| `POST /api/v1/gedcom/import` · `GET /gedcom/export` | editor/viewer | GEDCOM 5.5.1 |
| `GET /api/v1/plugins` · `PUT /plugins/{id}` | viewer/admin | manifests + UI slots; enable/disable |
| `* /api/v1/plugins/{id}/…` | per route | plugin-owned routes, 404 when disabled |

Errors: `{"error":{"code":"not_found","message":"…","fields":{…}}}`.

**Temporary passwords.** Admins can create users or reset passwords with
`mustChangePassword: true`. Until that user calls `POST /auth/change-password`, the
shared router refuses every non-public route except `GET /auth/me`,
`POST /auth/change-password` and `POST /auth/signout` with
`403 {"error":{"code":"password_change_required"}}`, and the SPA shows only the
Change Temporary Password screen.

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
| `/setup`, `/auth` | first-admin setup, sign-in (old `/login` links redirect); `/auth/callback` is reserved for identity-provider redirects |
| `/` | dashboard: counts, recent activity, quick actions |
| `/people`, `/people/new`, `/people/:id`, `/people/:id/edit` | directory and profile |
| `/tree`, `/tree/:id` | interactive ancestor/descendant tree (pan/zoom, expand/collapse, focus), rendered with React Flow and laid out with Dagre — see below |
| `/timeline` | chronological timeline with person/type/text filters |
| `/media` | media library |
| `/settings/:section` | account, users, backup & restore, import/export, activity, plugins |
| `/x/:pluginId/*` | plugin-contributed pages |

Tree view pipeline (`web/src/features/tree/graph.ts`, unit-tested):

1. The API's flat graph is merged with any "load more" sub-graphs (by person id and
   relationship id, so nothing is duplicated or overwritten).
2. `visibleGraph` walks from the root, skipping collapsed people; a shared ancestor
   stays visible while any other path reaches it.
3. `familyUnits` turns parent sets and couples into small junction nodes
   (parents → junction → children). Dagre cannot pin partners to the same rank, so
   the shared junction is what keeps couples side by side. Children with mixed
   birth/adoptive parents get separate edges so each keeps its style.
4. Dagre lays out fixed-size cards (`rankdir` `BT` for ancestors, `TB` for
   descendants); centre-anchored positions are shifted to React Flow's top-left anchor.
5. React Flow renders `PersonNode` / `UnionNode` (module-level `nodeTypes`) and fits
   the view whenever the root, direction or depth changes. The route is lazy-loaded,
   so React Flow and Dagre are only downloaded for the tree page.

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

Plugins get SQL access to their own tables through `store.PluginDB`
(`Query`, `QueryRow`, and `WriteTx`, which writes audit-log entries in the same
transaction). They may read core tables but only write their own.

### Compile-time registry and the On/Off switch

Each plugin lives in `plugins/<name>/` with its Go code, `migrations/*.sql` and
`web/` UI. Adding one means listing it in `plugins/plugins.go` and
`plugins/web.ts`; Vite resolves `@plugins/*` to that folder and dedupes React etc.
so plugin UIs share the app's libraries. **Settings → Plugins** turns a plugin
on or off (admins only); the choice is stored in `system_setting`
(`plugin.<id>.enabled`) and survives restarts. While a plugin is off, its routes
return `404 plugin_disabled`, its nav items and profile tabs are hidden, and its
data is kept. `HOLOCRON_DISABLED_PLUGINS` forces plugins off at the server level.

Restore never replaces `schema_migration` / `plugin_migration`: they describe the
live database's schema, so restoring a backup taken before a plugin existed
can't make the plugin re-run its migrations on the next start.

### Research Tasks (`plugins/task`)

Tables `plg_task_task` (title, `description_md`, status
`open|in_progress|blocked|done`, priority, `claimed_by_user_id`/`claimed_at`, the
attribution quartet), `plg_task_tag`, `plg_task_person` and `plg_task_media` (links
cascade when a person or media item is deleted), and `plg_task_comments` (research
notes: `task_id` cascading, `user_id` set null if the account is removed, `body_md`,
`edited`, timestamps). Routes under `/api/v1/plugins/task/`:

| Route | Notes |
|---|---|
| `GET/POST /tasks`, `GET/PUT/DELETE /tasks/{id}` | list items carry `commentCount` |
| `POST /tasks/{id}/claim` | `{"takeOver": true}` to take over someone else's claim; an open task becomes `in_progress` |
| `POST /tasks/{id}/unclaim` | claimer or admin; `in_progress` returns to `open` |
| `PATCH /tasks/{id}/status` | quick switch between `in_progress`, `blocked`, `done`; `409 not_claimed` if the task is unclaimed |
| `GET/POST /tasks/{id}/comments` | chronological; author is the current user |
| `PATCH/DELETE /tasks/{id}/comments/{commentId}` | author or admin only |
| `GET /tags` | tags in use with counts |

Every change (including claims, quick status changes and notes) is audited and
appears in each linked person's History. UI: a "Research tasks" nav page whose
cards have an inline status menu once claimed and a 💬 note count, a task detail
view with a research-notes thread, and a "Tasks" tab on person profiles.

A DNA plugin would add `plg_dna_*` tables (kits, matches, segments), expose
`/api/v1/plugins/dna/...`, contribute a `person.tab` ("DNA") and a `nav`
entry ("Chromosome browser"), and react to `person` deletions via
`ChangeListener` — without touching any core table or view. Plugin tables
live in the same SQLite file, so backups and restores carry them automatically.
