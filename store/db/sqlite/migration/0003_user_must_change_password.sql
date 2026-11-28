-- Temporary passwords: an admin-set password must be changed at the next sign-in.
-- Only accounts with a local password can be flagged. An empty password_hash means
-- "no local password" (reserved for accounts provisioned by an identity provider).
ALTER TABLE user ADD COLUMN must_change_password INTEGER NOT NULL DEFAULT 0
  CHECK (must_change_password IN (0, 1) AND (must_change_password = 0 OR password_hash <> ''));
