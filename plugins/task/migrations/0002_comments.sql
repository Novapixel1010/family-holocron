-- Research Tasks plugin, schema v2: a research-log thread per task.
-- (Prefixed like every plugin table so plugin data stays isolated from core tables.)
CREATE TABLE plg_task_comments (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id    INTEGER NOT NULL REFERENCES plg_task_task(id) ON DELETE CASCADE,
  -- SET NULL keeps a removed user's findings in the log (shown as "a removed user").
  user_id    INTEGER REFERENCES user(id) ON DELETE SET NULL,
  body_md    TEXT    NOT NULL,
  -- Explicit flag: timestamps have one-second resolution, so a quick edit could
  -- otherwise be indistinguishable from the original post.
  edited     INTEGER NOT NULL DEFAULT 0 CHECK (edited IN (0, 1)),
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX idx_plg_task_comments_task ON plg_task_comments(task_id, created_at);
