-- Add a "已转测" (tested/handed-to-QA) version status between 进行中 and 已发布.
-- SQLite cannot ALTER a CHECK constraint in place, so recreate the versions
-- table with the expanded status set. Data is preserved via a copy.

CREATE TABLE IF NOT EXISTS versions_new (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT    NOT NULL,
    description  TEXT    NOT NULL DEFAULT '',
    start_date   TEXT,
    release_date TEXT,
    status       TEXT    NOT NULL DEFAULT 'planned'
                         CHECK (status IN ('planned','active','tested','released')),
    created_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    vtype        TEXT    NOT NULL DEFAULT '',
    r_num        INTEGER,
    c_num        INTEGER,
    b_num        INTEGER,
    extra_num    INTEGER
);

INSERT INTO versions_new (id, name, description, start_date, release_date, status, created_at, updated_at, vtype, r_num, c_num, b_num, extra_num)
SELECT id, name, description, start_date, release_date, status, created_at, updated_at, vtype, r_num, c_num, b_num, extra_num FROM versions;

DROP TABLE versions;
ALTER TABLE versions_new RENAME TO versions;
