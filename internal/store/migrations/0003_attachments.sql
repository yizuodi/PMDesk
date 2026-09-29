-- Files attached to issues. Binary content lives on disk under attachments/;
-- this table records the metadata and the on-disk filename.
CREATE TABLE IF NOT EXISTS attachments (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id    INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    filename    TEXT    NOT NULL,                 -- user-facing original name
    stored      TEXT    NOT NULL,                 -- unique on-disk name
    size        INTEGER NOT NULL DEFAULT 0,
    content_type TEXT   NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_attachments_issue ON attachments(issue_id);
