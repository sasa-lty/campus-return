PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;
PRAGMA busy_timeout=5000;
CREATE TABLE IF NOT EXISTS records (
 kind TEXT NOT NULL,
 id TEXT NOT NULL,
 data TEXT NOT NULL CHECK(json_valid(data)),
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 PRIMARY KEY(kind,id)
);
CREATE INDEX IF NOT EXISTS by_user ON records(kind,json_extract(data,'$.userId'));
CREATE INDEX IF NOT EXISTS by_item ON records(kind,json_extract(data,'$.itemId'));
CREATE INDEX IF NOT EXISTS by_status ON records(kind,json_extract(data,'$.status'));
CREATE UNIQUE INDEX IF NOT EXISTS unique_username ON records(json_extract(data,'$.username')) WHERE kind='user';
CREATE TABLE IF NOT EXISTS operations (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL,
 idem TEXT NOT NULL,
 request_hash TEXT NOT NULL,
 data TEXT NOT NULL CHECK(json_valid(data)),
 UNIQUE(user_id,idem)
);
CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY,applied_at TEXT NOT NULL);
INSERT OR IGNORE INTO schema_migrations VALUES(1,strftime('%Y-%m-%dT%H:%M:%fZ','now'));
