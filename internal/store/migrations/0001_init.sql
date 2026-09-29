-- Initial schema for PM Jira (personal project management)

-- Project settings (singleton; row id=1 always exists)
CREATE TABLE IF NOT EXISTS projects (
    id          INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL DEFAULT 'My Project',
    pkey        TEXT    NOT NULL DEFAULT 'PM',
    issue_seq   INTEGER NOT NULL DEFAULT 0
);

-- Versions / releases
CREATE TABLE IF NOT EXISTS versions (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT    NOT NULL,
    description  TEXT    NOT NULL DEFAULT '',
    start_date   TEXT,                          -- ISO date YYYY-MM-DD, nullable
    release_date TEXT,                          -- ISO date YYYY-MM-DD, nullable
    status       TEXT    NOT NULL DEFAULT 'planned'
                         CHECK (status IN ('planned','active','tested','released')),
    created_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- Issues / work items
CREATE TABLE IF NOT EXISTS issues (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    seq         INTEGER NOT NULL,               -- project-wide number, e.g. 123
    key         TEXT    NOT NULL UNIQUE,        -- e.g. PM-123
    title       TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    type        TEXT    NOT NULL DEFAULT 'task'
                         CHECK (type IN ('epic','story','task','bug')),
    status      TEXT    NOT NULL DEFAULT 'todo'
                         CHECK (status IN ('todo','in_progress','in_review','done')),
    priority    TEXT    NOT NULL DEFAULT 'medium'
                         CHECK (priority IN ('lowest','low','medium','high','highest')),
    assignee    TEXT    NOT NULL DEFAULT '',
    version_id  INTEGER REFERENCES versions(id) ON DELETE SET NULL,
    epic_id     INTEGER REFERENCES issues(id)   ON DELETE SET NULL,
    due_date    TEXT,                           -- ISO date YYYY-MM-DD, nullable
    board_order INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_issues_status  ON issues(status);
CREATE INDEX IF NOT EXISTS idx_issues_type    ON issues(type);
CREATE INDEX IF NOT EXISTS idx_issues_version ON issues(version_id);
CREATE INDEX IF NOT EXISTS idx_issues_epic    ON issues(epic_id);
CREATE INDEX IF NOT EXISTS idx_issues_due     ON issues(due_date);

-- Comments
CREATE TABLE IF NOT EXISTS comments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id   INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    body       TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_comments_issue ON comments(issue_id);

-- Activity / change history
CREATE TABLE IF NOT EXISTS activities (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id   INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    field      TEXT    NOT NULL,                -- e.g. 'status', 'priority'
    old_value  TEXT    NOT NULL DEFAULT '',
    new_value  TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_activities_issue ON activities(issue_id);

-- Seed singleton project row if absent
INSERT OR IGNORE INTO projects (id, name, pkey, issue_seq) VALUES (1, 'My Project', 'PM', 0);
