-- Personal access tokens (modeled on usememos/memos). Only the SHA-256 hex digest
-- of a token is stored; the plaintext is shown to its owner once, at creation.
-- Timestamps are unix seconds; a NULL expires_at never expires.
CREATE TABLE user_access_tokens (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
  description  TEXT    NOT NULL CHECK (length(trim(description)) > 0),
  token_hash   TEXT    NOT NULL UNIQUE,
  expires_at   INTEGER,
  last_used_at INTEGER,
  created_at   INTEGER NOT NULL
);
CREATE INDEX idx_user_access_tokens_user ON user_access_tokens(user_id);
