-- Remove obsolete issue relationships now that issues and versions are
-- independent features. The migration runner disables foreign keys for this
-- controlled SQLite table swap and checks them again before committing.

CREATE TABLE issues_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    seq         INTEGER NOT NULL,
    key         TEXT    NOT NULL UNIQUE,
    title       TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    type        TEXT    NOT NULL DEFAULT 'task'
                         CHECK (type IN ('epic','story','task','bug')),
    status      TEXT    NOT NULL DEFAULT 'todo'
                         CHECK (status IN ('todo','in_progress','in_review','done')),
    priority    TEXT    NOT NULL DEFAULT 'medium'
                         CHECK (priority IN ('lowest','low','medium','high','highest')),
    due_date    TEXT,
    board_order INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

INSERT INTO issues_new (id, seq, key, title, description, type, status, priority,
                        due_date, board_order, created_at, updated_at)
SELECT id, seq, key, title, description, type, status, priority,
       due_date, board_order, created_at, updated_at
FROM issues;

DROP TABLE issues;
ALTER TABLE issues_new RENAME TO issues;

CREATE INDEX idx_issues_status ON issues(status);
CREATE INDEX idx_issues_type   ON issues(type);
CREATE INDEX idx_issues_due    ON issues(due_date);
