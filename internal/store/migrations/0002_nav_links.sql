-- Custom quick-navigation links shown in the top bar.
-- mode 'iframe' embeds the URL inside the app; 'newtab' opens it externally.
CREATE TABLE IF NOT EXISTS nav_links (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT    NOT NULL,
    url        TEXT    NOT NULL,
    mode       TEXT    NOT NULL DEFAULT 'newtab'
                         CHECK (mode IN ('iframe','newtab')),
    position   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
