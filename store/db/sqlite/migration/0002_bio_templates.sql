-- Admin-managed Markdown templates for a person's About / biography section.
CREATE TABLE bio_templates (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  name               TEXT    NOT NULL UNIQUE COLLATE NOCASE,
  description        TEXT    NOT NULL DEFAULT '',
  content_md         TEXT    NOT NULL,
  sort_order         INTEGER NOT NULL DEFAULT 0,
  created_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  updated_by_user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL
);
CREATE INDEX idx_bio_templates_order ON bio_templates(sort_order, name COLLATE NOCASE);
