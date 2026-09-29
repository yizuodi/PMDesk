package store

import (
	"fmt"
	"time"
)

// GetProject returns the singleton project/settings row.
func (s *Store) GetProject() (Project, error) {
	row := s.DB.QueryRowContext(ctx(5*time.Second), `SELECT id, name, pkey, issue_seq FROM projects WHERE id = 1`)
	var p Project
	if err := row.Scan(&p.ID, &p.Name, &p.Key, &p.IssueSeq); err != nil {
		return Project{}, fmt.Errorf("get project: %w", err)
	}
	return p, nil
}

// UpdateProject updates the project name and key prefix.
func (s *Store) UpdateProject(name, key string) error {
	_, err := s.DB.ExecContext(ctx(5*time.Second),
		`UPDATE projects SET name = ?, pkey = upper(?) WHERE id = 1`, name, key)
	return err
}

// nextIssueKey allocates the next sequence number and key for a new issue.
// Runs inside a transaction to keep seq/key consistent.
func (s *Store) nextIssueKey() (int, string, error) {
	tx, err := s.DB.BeginTx(ctx(10*time.Second), nil)
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback()

	var pkey string
	var seq int
	if err := tx.QueryRow(`SELECT pkey, issue_seq FROM projects WHERE id = 1`).Scan(&pkey, &seq); err != nil {
		return 0, "", err
	}
	seq++
	if _, err := tx.Exec(`UPDATE projects SET issue_seq = ? WHERE id = 1`, seq); err != nil {
		return 0, "", err
	}
	if err := tx.Commit(); err != nil {
		return 0, "", err
	}
	return seq, fmt.Sprintf("%s-%d", pkey, seq), nil
}
