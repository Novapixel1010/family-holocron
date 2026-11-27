-- Family Holocron core schema v1.
-- Timestamps are unix seconds. Attribution user FKs use ON DELETE SET NULL so
-- history survives account removal; audit_log keeps a username snapshot.

CREATE TABLE user (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  username      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
  display_name  TEXT    NOT NULL DEFAULT '',
  email         TEXT    NOT NULL DEFAULT '',
  password_hash TEXT    NOT NULL,
  role          TEXT    NOT NULL DEFAULT 'editor' CHECK (role IN ('admin', 'editor', 'viewer')),
  status        TEXT    NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL
);

CREATE TABLE session (
  token_hash   TEXT    PRIMARY KEY,
  user_id      INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
  created_at   INTEGER NOT NULL,
  expires_at   INTEGER NOT NULL,
  last_seen_at INTEGER NOT NULL,
  user_agent   TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX idx_session_user ON session(user_id);

CREATE TABLE media (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  uid                TEXT    NOT NULL UNIQUE,
  storage_path       TEXT    NOT NULL UNIQUE,
  original_name      TEXT    NOT NULL DEFAULT '',
  mime_type          TEXT    NOT NULL DEFAULT 'application/octet-stream',
  size_bytes         INTEGER NOT NULL DEFAULT 0,
  sha256             TEXT    NOT NULL DEFAULT '',
  title              TEXT    NOT NULL DEFAULT '',
  description        TEXT    NOT NULL DEFAULT '',
  taken_date         TEXT    NOT NULL DEFAULT '',
  created_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  updated_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL
);

CREATE TABLE person (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  uid                TEXT    NOT NULL UNIQUE,
  given_name         TEXT    NOT NULL DEFAULT '',
  surname            TEXT    NOT NULL DEFAULT '',
  nickname           TEXT    NOT NULL DEFAULT '',
  sex                TEXT    NOT NULL DEFAULT 'U' CHECK (sex IN ('M', 'F', 'X', 'U')),
  birth_date         TEXT    NOT NULL DEFAULT '',
  birth_sort         TEXT    NOT NULL DEFAULT '',
  birth_place        TEXT    NOT NULL DEFAULT '',
  death_date         TEXT    NOT NULL DEFAULT '',
  death_sort         TEXT    NOT NULL DEFAULT '',
  death_place        TEXT    NOT NULL DEFAULT '',
  deceased           INTEGER NOT NULL DEFAULT 0 CHECK (deceased IN (0, 1)),
  about              TEXT    NOT NULL DEFAULT '',
  portrait_media_id  INTEGER REFERENCES media(id) ON DELETE SET NULL,
  created_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  updated_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL
);
CREATE INDEX idx_person_name ON person(surname COLLATE NOCASE, given_name COLLATE NOCASE);
CREATE INDEX idx_person_birth ON person(birth_sort);

CREATE TABLE relationship (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  type               TEXT    NOT NULL CHECK (type IN ('parent', 'spouse')),
  person1_id         INTEGER NOT NULL REFERENCES person(id) ON DELETE CASCADE,
  person2_id         INTEGER NOT NULL REFERENCES person(id) ON DELETE CASCADE,
  subtype            TEXT    NOT NULL DEFAULT '',
  start_date         TEXT    NOT NULL DEFAULT '',
  end_date           TEXT    NOT NULL DEFAULT '',
  notes              TEXT    NOT NULL DEFAULT '',
  created_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  updated_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL,
  CHECK (person1_id <> person2_id),
  UNIQUE (type, person1_id, person2_id)
);
CREATE INDEX idx_relationship_p2 ON relationship(person2_id, type);

CREATE TABLE event (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  type               TEXT    NOT NULL,
  title              TEXT    NOT NULL DEFAULT '',
  date               TEXT    NOT NULL DEFAULT '',
  sort_date          TEXT    NOT NULL DEFAULT '',
  place              TEXT    NOT NULL DEFAULT '',
  description        TEXT    NOT NULL DEFAULT '',
  relationship_id    INTEGER REFERENCES relationship(id) ON DELETE SET NULL,
  created_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  updated_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL
);
CREATE INDEX idx_event_sort ON event(sort_date);

CREATE TABLE event_participant (
  event_id  INTEGER NOT NULL REFERENCES event(id) ON DELETE CASCADE,
  person_id INTEGER NOT NULL REFERENCES person(id) ON DELETE CASCADE,
  role      TEXT    NOT NULL DEFAULT 'principal',
  PRIMARY KEY (event_id, person_id)
);
CREATE INDEX idx_event_participant_person ON event_participant(person_id);

CREATE TABLE note (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  person_id          INTEGER NOT NULL REFERENCES person(id) ON DELETE CASCADE,
  title              TEXT    NOT NULL DEFAULT '',
  body               TEXT    NOT NULL DEFAULT '',
  created_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  updated_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL
);
CREATE INDEX idx_note_person ON note(person_id);

CREATE TABLE media_person (
  media_id  INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
  person_id INTEGER NOT NULL REFERENCES person(id) ON DELETE CASCADE,
  PRIMARY KEY (media_id, person_id)
);
CREATE INDEX idx_media_person_person ON media_person(person_id);

CREATE TABLE audit_log (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at   INTEGER NOT NULL,
  user_id      INTEGER REFERENCES user(id) ON DELETE SET NULL,
  username     TEXT    NOT NULL DEFAULT '',
  action       TEXT    NOT NULL,
  entity_type  TEXT    NOT NULL,
  entity_id    INTEGER NOT NULL DEFAULT 0,
  entity_label TEXT    NOT NULL DEFAULT '',
  person_ids   TEXT    NOT NULL DEFAULT '[]',
  changes      TEXT    NOT NULL DEFAULT '{}',
  summary      TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX idx_audit_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_user ON audit_log(user_id);

-- Per-person history lookup (audit rows can concern several people).
CREATE TABLE audit_person (
  audit_id  INTEGER NOT NULL REFERENCES audit_log(id) ON DELETE CASCADE,
  person_id INTEGER NOT NULL,
  PRIMARY KEY (audit_id, person_id)
);
CREATE INDEX idx_audit_person_person ON audit_person(person_id);

CREATE TABLE system_setting (
  name       TEXT    PRIMARY KEY,
  value      TEXT    NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE plugin_kv (
  plugin_id  TEXT    NOT NULL,
  key        TEXT    NOT NULL,
  value      TEXT    NOT NULL,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY (plugin_id, key)
);

CREATE TABLE plugin_migration (
  plugin_id  TEXT    NOT NULL,
  version    INTEGER NOT NULL,
  applied_at INTEGER NOT NULL,
  PRIMARY KEY (plugin_id, version)
);
