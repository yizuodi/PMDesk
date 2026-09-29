package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "nested", "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, dir
}

func createSchemaVersionSeven(t *testing.T, dbPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	dsn := "file:" + dbPath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for n := 1; n <= 7; n++ {
		name := fmt.Sprintf("migrations/%04d_", n)
		var path string
		names, globErr := fsGlobMigrationFiles()
		if globErr != nil {
			t.Fatal(globErr)
		}
		for _, candidate := range names {
			if len(candidate) >= len(name) && candidate[:len(name)] == name {
				path = candidate
				break
			}
		}
		if path == "" {
			t.Fatalf("migration %04d not found", n)
		}
		content, readErr := migrationFS.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, execErr := db.Exec(string(content)); execErr != nil {
			t.Fatalf("apply %s: %v", path, execErr)
		}
	}
	if _, err := db.Exec(`INSERT INTO meta(key,value) VALUES('schema_version','7')`); err != nil {
		t.Fatal(err)
	}
}

func fsGlobMigrationFiles() ([]string, error) {
	return fs.Glob(migrationFS, "migrations/*.sql")
}

func TestApplyMigrationRollsBackAndRestoresForeignKeys(t *testing.T) {
	s, _ := openTestStore(t)
	conn, err := s.DB.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()

	content := []byte(`CREATE TABLE migration_probe (id INTEGER PRIMARY KEY);
		INSERT INTO migration_probe VALUES (1);
		THIS IS NOT VALID SQL;`)
	if err := applyMigration(conn, content, 999); err == nil {
		t.Fatal("invalid migration unexpectedly succeeded")
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	conn = nil
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='migration_probe'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed migration left its table behind")
	}
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM meta WHERE value='999'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed migration advanced schema version")
	}
	var foreignKeys int
	if err := s.DB.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d after failed migration, want 1", foreignKeys)
	}
}

func TestOpenInitializesAndReopensStore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nested", "pmjira.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "nested", "attachments")); err != nil {
		t.Fatalf("attachments directory: %v", err)
	}
	var version string
	if err := s.DB.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != "8" {
		t.Fatalf("schema version = %q, want 8", version)
	}
	var foreignKeys int
	if err := s.DB.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("project count = %d, want 1", count)
	}
}

func TestDecouplingMigrationPreservesIssueChildren(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")
	createSchemaVersionSeven(t, dbPath)

	legacy, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	legacy.SetMaxOpenConns(1)
	defer legacy.Close()
	if _, err := legacy.Exec(`
		INSERT INTO versions(name, description, start_date, status, vtype) VALUES('R027C10B001','旧版本','2027-01-01','active','B');
		INSERT INTO issues(seq,key,title,description,type,status,priority,assignee,version_id,epic_id,due_date)
		VALUES(1,'PM-1','保留的事务','内容','task','todo','medium','旧负责人',1,NULL,'2030-01-02');
		INSERT INTO comments(issue_id,body) VALUES(1,'评论');
		INSERT INTO activities(issue_id,field,old_value,new_value) VALUES(1,'version','1','2');
	`); err != nil {
		t.Fatal(err)
	}
	stored := NewStoredName("report.txt")
	if _, err := legacy.Exec(`INSERT INTO attachments(issue_id, filename, stored, size, content_type) VALUES(?,?,?,?,?)`, 1, "report.txt", stored, 4, "text/plain"); err != nil {
		t.Fatal(err)
	}
	legacy.Close()

	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	issue, err := s.GetIssue(1)
	if err != nil {
		t.Fatal(err)
	}
	if issue.Key != "PM-1" || issue.Title != "保留的事务" || issue.Description != "内容" || issue.DueDate == nil {
		t.Fatalf("retained issue = %#v", issue)
	}
	var columns []string
	rows, err := s.DB.Query(`PRAGMA table_info(issues)`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, name)
	}
	rows.Close()
	for _, removed := range []string{"assignee", "version_id", "epic_id"} {
		for _, column := range columns {
			if column == removed {
				t.Fatalf("legacy column %q remains", removed)
			}
		}
	}
	for _, removedIndex := range []string{"idx_issues_version", "idx_issues_epic"} {
		var count int
		if err := s.DB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, removedIndex).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("legacy index %q remains", removedIndex)
		}
	}

	for _, table := range []string{"comments", "activities", "attachments"} {
		var count int
		if err := s.DB.QueryRow(`SELECT COUNT(*) FROM ` + table + ` WHERE issue_id=1`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("%s rows after migration = %d, want 1", table, count)
		}
	}
	var violations int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil {
		t.Fatal(err)
	}
	if violations != 0 {
		t.Fatalf("foreign key violations = %d", violations)
	}

	if err := s.DeleteIssue(1); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"comments", "activities", "attachments"} {
		var count int
		if err := s.DB.QueryRow(`SELECT COUNT(*) FROM ` + table + ` WHERE issue_id=1`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s rows remain after issue deletion: %d", table, count)
		}
	}
}
