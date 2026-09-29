package store

import (
	"fmt"
	"time"
)

// AddComment attaches a comment to an issue.
func (s *Store) AddComment(issueID int64, body string) (Comment, error) {
	res, err := s.DB.ExecContext(ctx(5*time.Second),
		`INSERT INTO comments (issue_id, body) VALUES (?, ?)`, issueID, body)
	if err != nil {
		return Comment{}, err
	}
	id, _ := res.LastInsertId()
	row := s.DB.QueryRowContext(ctx(5*time.Second),
		`SELECT id, issue_id, body, created_at FROM comments WHERE id=?`, id)
	var c Comment
	if err := row.Scan(&c.ID, &c.IssueID, &c.Body, &c.CreatedAt); err != nil {
		return Comment{}, fmt.Errorf("load comment: %w", err)
	}
	return c, nil
}

// ListComments returns comments for an issue, newest last.
func (s *Store) ListComments(issueID int64) ([]Comment, error) {
	rows, err := s.DB.QueryContext(ctx(5*time.Second),
		`SELECT id, issue_id, body, created_at FROM comments WHERE issue_id=? ORDER BY id ASC`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.IssueID, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteComment removes a comment.
func (s *Store) DeleteComment(id int64) error {
	_, err := s.DB.ExecContext(ctx(5*time.Second), `DELETE FROM comments WHERE id=?`, id)
	return err
}

// ListActivities returns the change history for an issue, newest first.
func (s *Store) ListActivities(issueID int64) ([]Activity, error) {
	rows, err := s.DB.QueryContext(ctx(5*time.Second),
		`SELECT id, issue_id, field, old_value, new_value, created_at
		 FROM activities WHERE issue_id=? ORDER BY id DESC`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Activity
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.IssueID, &a.Field, &a.OldValue, &a.NewValue, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
