package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"
)

// Attachment is a file attached to an issue.
type Attachment struct {
	ID          int64
	IssueID     int64
	Filename    string // original, user-facing name
	Stored      string // on-disk filename
	Size        int64
	ContentType string
	CreatedAt   string
}

// NewStoredName returns a unique on-disk filename for an upload, keeping the
// original extension and sanitising the base name.
func NewStoredName(original string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	prefix := hex.EncodeToString(b)
	ext := filepath.Ext(original)
	return prefix + ext
}

// AttachmentPath returns the full filesystem path for a stored filename.
func (s *Store) AttachmentPath(stored string) string {
	return filepath.Join(s.AttachDir, stored)
}

// CreateAttachment records a newly saved attachment.
func (s *Store) CreateAttachment(issueID int64, filename, stored string, size int64, contentType string) (Attachment, error) {
	res, err := s.DB.ExecContext(ctx(5*time.Second),
		`INSERT INTO attachments (issue_id, filename, stored, size, content_type) VALUES (?,?,?,?,?)`,
		issueID, filename, stored, size, contentType)
	if err != nil {
		return Attachment{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetAttachment(id)
}

// GetAttachment returns one attachment by id.
func (s *Store) GetAttachment(id int64) (Attachment, error) {
	var a Attachment
	err := s.DB.QueryRowContext(ctx(5*time.Second),
		`SELECT id, issue_id, filename, stored, size, content_type, created_at FROM attachments WHERE id=?`, id).
		Scan(&a.ID, &a.IssueID, &a.Filename, &a.Stored, &a.Size, &a.ContentType, &a.CreatedAt)
	if err != nil {
		return Attachment{}, fmt.Errorf("get attachment: %w", err)
	}
	return a, nil
}

// ListAttachments returns all attachments for an issue, newest first.
func (s *Store) ListAttachments(issueID int64) ([]Attachment, error) {
	rows, err := s.DB.QueryContext(ctx(5*time.Second),
		`SELECT id, issue_id, filename, stored, size, content_type, created_at
		 FROM attachments WHERE issue_id=? ORDER BY id DESC`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attachment
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.IssueID, &a.Filename, &a.Stored, &a.Size, &a.ContentType, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DeleteAttachment removes the row and returns the stored filename so the
// caller can delete the file from disk.
func (s *Store) DeleteAttachment(id int64) (string, error) {
	var stored string
	err := s.DB.QueryRowContext(ctx(5*time.Second),
		`SELECT stored FROM attachments WHERE id=?`, id).Scan(&stored)
	if err != nil {
		return "", err
	}
	_, err = s.DB.ExecContext(ctx(5*time.Second), `DELETE FROM attachments WHERE id=?`, id)
	return stored, err
}

// ListAttachmentFiles returns the stored filenames for an issue (used to clean
// up files when an issue is deleted).
func (s *Store) ListAttachmentFiles(issueID int64) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx(5*time.Second),
		`SELECT stored FROM attachments WHERE issue_id=?`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var st string
		if err := rows.Scan(&st); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}
