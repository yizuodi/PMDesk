// Package store provides the SQLite-backed data layer for PM Jira.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Store wraps the SQLite database handle.
type Store struct {
	DB        *sql.DB
	AttachDir string // directory holding uploaded attachment files
}

// Open opens (or creates) the SQLite database at dbPath, applies pending
// migrations, and returns a ready-to-use Store. foreign_keys are enabled so
// that ON DELETE CASCADE / SET NULL behave as declared in the schema.
func Open(dbPath string) (*Store, error) {
	// Ensure the parent directory exists (e.g. when the db path is nested).
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}

	dsn := "file:" + dbPath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// A single connection avoids "database is locked" under concurrent writes,
	// and is plenty for a personal tool. WAL keeps reads non-blocking.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx(5 * time.Second)); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{DB: db}
	if err := s.migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	// Attachments live next to the database file.
	s.AttachDir = filepath.Join(filepath.Dir(dbPath), "attachments")
	if err := os.MkdirAll(s.AttachDir, 0o755); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create attachments dir: %w", err)
	}
	return s, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.DB.Close() }

// migrate applies numbered SQL files from migrations/ in order, tracking the
// applied version in a meta table. Each migration and its schema-version record
// are committed atomically, with foreign-key checks performed before commit.
func (s *Store) migrate(db *sql.DB) error {
	conn, err := db.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("get migration connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(context.Background(), `CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create meta: %w", err)
	}
	var current int
	row := conn.QueryRowContext(context.Background(), `SELECT value FROM meta WHERE key = 'schema_version'`)
	switch scanErr := row.Scan(&current); {
	case scanErr == sql.ErrNoRows:
		current = 0
	case scanErr != nil:
		return scanErr
	}

	names, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	for _, name := range names {
		version := parseMigrationVersion(name)
		if version <= current {
			continue
		}
		content, err := migrationFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if err := applyMigration(conn, content, version); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		current = version
	}
	return nil
}

func applyMigration(conn *sql.Conn, content []byte, version int) (err error) {
	ctx := context.Background()
	if _, err = conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("disable foreign keys: %w", err)
	}

	var tx *sql.Tx
	committed := false
	defer func() {
		if tx != nil && !committed {
			_ = tx.Rollback()
		}
		if _, restoreErr := conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err == nil && restoreErr != nil {
			err = fmt.Errorf("restore foreign keys: %w", restoreErr)
		}
	}()

	tx, err = conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if _, err = tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("execute migration: %w", err)
	}
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO meta(key,value) VALUES('schema_version', ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		fmt.Sprintf("%d", version)); err != nil {
		return fmt.Errorf("record schema version: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("foreign key check: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("foreign key violation")
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("foreign key check rows: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	committed = true
	return nil
}

// parseMigrationVersion reads the leading numeric segment of "migrations/0001_init.sql".
func parseMigrationVersion(name string) int {
	base := name
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	n := 0
	for _, r := range base {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// ctx returns a short-lived context for a single DB call. The cancel func is
// released automatically once the timeout elapses (or the call completes).
func ctx(d time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	time.AfterFunc(d, cancel)
	return ctx
}

// ---- Model types ----------------------------------------------------------

// Project is the singleton settings row.
type Project struct {
	ID       int
	Name     string
	Key      string // prefix used in issue keys, e.g. "PM"
	IssueSeq int    // last issued sequence number
}

// Version is defined in versions.go (it carries the structured scheme fields).

// Issue is a tracked work item.
type Issue struct {
	ID          int64
	Seq         int
	Key         string
	Title       string
	Description string
	Type        string  // epic | story | task | bug
	Status      string  // todo | in_progress | in_review | done
	Priority    string  // lowest | low | medium | high | highest
	DueDate     *string // "YYYY-MM-DD" or nil
	BoardOrder  int
	CreatedAt   string
	UpdatedAt   string

	// Computed field populated by issue list queries.
	CommentCount int
}

// Comment is a note attached to an issue.
type Comment struct {
	ID        int64
	IssueID   int64
	Body      string
	CreatedAt string
}

// Activity records a single field change on an issue.
type Activity struct {
	ID        int64
	IssueID   int64
	Field     string
	OldValue  string
	NewValue  string
	CreatedAt string
}
