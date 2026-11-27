-- Research Tasks plugin, schema v1. All tables are prefixed plg_task_ and live in the
-- main SQLite file, so backups and restores include them automatically.

CREATE TABLE plg_task_task (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  title              TEXT    NOT NULL,
  description_md     TEXT    NOT NULL DEFAULT '',
  status             TEXT    NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'in_progress', 'blocked', 'done')),
  priority           TEXT    NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high')),
  claimed_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  claimed_at         INTEGER,
  created_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  updated_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL
);
CREATE INDEX idx_plg_task_task_status ON plg_task_task(status, priority);
CREATE INDEX idx_plg_task_task_claimed ON plg_task_task(claimed_by_user_id);

CREATE TABLE plg_task_tag (
  task_id INTEGER NOT NULL REFERENCES plg_task_task(id) ON DELETE CASCADE,
  tag     TEXT    NOT NULL COLLATE NOCASE,
  PRIMARY KEY (task_id, tag)
);
CREATE INDEX idx_plg_task_tag_tag ON plg_task_tag(tag);

-- Links to core records; deleting a person or media item just removes the link.
CREATE TABLE plg_task_person (
  task_id   INTEGER NOT NULL REFERENCES plg_task_task(id) ON DELETE CASCADE,
  person_id INTEGER NOT NULL REFERENCES person(id) ON DELETE CASCADE,
  PRIMARY KEY (task_id, person_id)
);
CREATE INDEX idx_plg_task_person_person ON plg_task_person(person_id);

CREATE TABLE plg_task_media (
  task_id  INTEGER NOT NULL REFERENCES plg_task_task(id) ON DELETE CASCADE,
  media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
  PRIMARY KEY (task_id, media_id)
);
CREATE INDEX idx_plg_task_media_media ON plg_task_media(media_id);
