-- 人员速查模块：人员 + 接口人链接清单。
-- roles 以逗号分隔存储（UI 用标签编辑器逐个增删）。
CREATE TABLE IF NOT EXISTS people (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL,
    employee_id TEXT    NOT NULL DEFAULT '',   -- 工号，可能含字母
    roles       TEXT    NOT NULL DEFAULT '',   -- 逗号分隔的角色标签
    team        TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_people_team ON people(team);
CREATE INDEX IF NOT EXISTS idx_people_employee ON people(employee_id);

-- 接口人列表：名称 + URL + 备注 的超链接清单
CREATE TABLE IF NOT EXISTS contact_links (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    url        TEXT    NOT NULL,
    note       TEXT    NOT NULL DEFAULT '',
    position   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
