package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// IssueFilter narrows an issue query. Zero values mean "no filter".
type IssueFilter struct {
	Search    string // matches key or title (ILIKE)
	Type      string // epic|story|task|bug
	Status    string // todo|in_progress|in_review|done
	Priority  string
	Overdue   bool // due_date < today and not done
	DueWithin int  // >0 = due within N days (and not done)
}

// whereBuilder assembles a parameterised WHERE clause for issue filters.
func (f IssueFilter) where() (string, []any) {
	var clauses []string
	var args []any
	add := func(clause string, vals ...any) {
		clauses = append(clauses, clause)
		args = append(args, vals...)
	}
	if f.Search != "" {
		add("(i.key LIKE ? OR i.title LIKE ?)", "%"+f.Search+"%", "%"+f.Search+"%")
	}
	if f.Type != "" {
		add("i.type = ?", f.Type)
	}
	if f.Status != "" {
		add("i.status = ?", f.Status)
	}
	if f.Priority != "" {
		add("i.priority = ?", f.Priority)
	}
	if f.Overdue {
		add("i.due_date IS NOT NULL AND i.due_date < ? AND i.status <> 'done'", today())
	}
	if f.DueWithin > 0 {
		add("i.due_date IS NOT NULL AND i.status <> 'done' AND i.due_date >= ? AND i.due_date <= ?",
			today(), todayPlus(f.DueWithin))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func today() string          { return time.Now().Format("2006-01-02") }
func todayPlus(n int) string { return time.Now().AddDate(0, 0, n).Format("2006-01-02") }

// issueCols is the canonical column list for plain issue reads.
const issueCols = `id, seq, key, title, description, type, status, priority,
	due_date, board_order, created_at, updated_at`

func scanIssue(row interface {
	Scan(dest ...any) error
}) (Issue, error) {
	var i Issue
	var dueDate sql.NullString
	if err := row.Scan(
		&i.ID, &i.Seq, &i.Key, &i.Title, &i.Description, &i.Type, &i.Status, &i.Priority,
		&dueDate, &i.BoardOrder, &i.CreatedAt, &i.UpdatedAt,
	); err != nil {
		return Issue{}, err
	}
	if dueDate.Valid {
		s := dueDate.String
		i.DueDate = &s
	}
	return i, nil
}

// ListIssues returns issues matching the filter, ordered appropriately for
// the list view and including comment counts.
func (s *Store) ListIssues(f IssueFilter) ([]Issue, error) {
	where, args := f.where()
	q := `SELECT i.id, i.seq, i.key, i.title, '', i.type, i.status, i.priority,
			i.due_date, i.board_order, i.created_at, i.updated_at,
			(SELECT COUNT(*) FROM comments c WHERE c.issue_id = i.id)
		FROM issues i
		` + where + `
		ORDER BY i.due_date IS NULL, i.due_date ASC, i.seq DESC`
	rows, err := s.DB.QueryContext(ctx(5*time.Second), q, args...)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()

	var out []Issue
	for rows.Next() {
		var i Issue
		var dueDate sql.NullString
		if err := rows.Scan(
			&i.ID, &i.Seq, &i.Key, &i.Title, new(string), &i.Type, &i.Status, &i.Priority,
			&dueDate, &i.BoardOrder, &i.CreatedAt, &i.UpdatedAt, &i.CommentCount,
		); err != nil {
			return nil, err
		}
		if dueDate.Valid {
			d := dueDate.String
			i.DueDate = &d
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// BoardColumns groups issues by status for the kanban board.
func (s *Store) BoardColumns() (map[string][]Issue, error) {
	issues, err := s.ListIssues(IssueFilter{})
	if err != nil {
		return nil, err
	}
	cols := map[string][]Issue{"todo": {}, "in_progress": {}, "in_review": {}, "done": {}}
	for _, i := range issues {
		cols[i.Status] = append(cols[i.Status], i)
	}
	return cols, nil
}

// GetIssue returns a single issue by id.
func (s *Store) GetIssue(id int64) (Issue, error) {
	row := s.DB.QueryRowContext(ctx(5*time.Second),
		`SELECT `+issueCols+` FROM issues WHERE id = ?`, id)
	return scanIssue(row)
}

// IssueInput holds the user-supplied fields when creating or updating an issue.
type IssueInput struct {
	Title       string
	Type        string
	Status      string
	Priority    string
	Description string
	DueDate     string // "YYYY-MM-DD" or ""
	BoardOrder  int
}

// CreateIssue inserts a new issue, allocating its seq/key from the project counter.
func (s *Store) CreateIssue(in IssueInput) (Issue, error) {
	seq, key, err := s.nextIssueKey()
	if err != nil {
		return Issue{}, err
	}
	due := nullString(in.DueDate)
	res, err := s.DB.ExecContext(ctx(5*time.Second), `INSERT INTO issues
		(seq, key, title, description, type, status, priority, due_date, board_order)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		seq, key, in.Title, in.Description, in.Type, in.Status, in.Priority,
		due, in.BoardOrder)
	if err != nil {
		return Issue{}, fmt.Errorf("create issue: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetIssue(id)
}

// UpdateIssue replaces an issue's editable fields and records field changes
// in the activity log.
func (s *Store) UpdateIssue(id int64, in IssueInput) (Issue, error) {
	before, err := s.GetIssue(id)
	if err != nil {
		return Issue{}, err
	}
	due := nullString(in.DueDate)
	_, err = s.DB.ExecContext(ctx(5*time.Second), `UPDATE issues SET
		title=?, type=?, status=?, priority=?, description=?, due_date=?,
		updated_at=strftime('%Y-%m-%dT%H:%M:%SZ','now') WHERE id=?`,
		in.Title, in.Type, in.Status, in.Priority, in.Description, due, id)
	if err != nil {
		return Issue{}, fmt.Errorf("update issue: %w", err)
	}
	s.logChanges(id, before, in)
	return s.GetIssue(id)
}

// SetIssueStatus moves an issue to a new status (used by the kanban board).
func (s *Store) SetIssueStatus(id int64, status string) error {
	before, err := s.GetIssue(id)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx(5*time.Second),
		`UPDATE issues SET status=?, updated_at=strftime('%Y-%m-%dT%H:%M:%SZ','now') WHERE id=?`,
		status, id)
	if err != nil {
		return err
	}
	if before.Status != status {
		s.recordActivity(id, "status", before.Status, status)
	}
	return nil
}

// DeleteIssue removes an issue (comments, activities, and attachments cascade).
func (s *Store) DeleteIssue(id int64) error {
	_, err := s.DB.ExecContext(ctx(5*time.Second), `DELETE FROM issues WHERE id=?`, id)
	return err
}

// logChanges records an activity entry for each field that changed.
func (s *Store) logChanges(id int64, before Issue, after IssueInput) {
	changes := []struct {
		field, oldV, newV string
	}{
		{"title", before.Title, after.Title},
		{"type", before.Type, after.Type},
		{"status", before.Status, after.Status},
		{"priority", before.Priority, after.Priority},
		{"description", before.Description, after.Description},
		{"due_date", ptrStr(before.DueDate), after.DueDate},
	}
	for _, c := range changes {
		if c.oldV != c.newV {
			s.recordActivity(id, c.field, c.oldV, c.newV)
		}
	}
}

func (s *Store) recordActivity(issueID int64, field, oldV, newV string) {
	_, _ = s.DB.ExecContext(ctx(5*time.Second),
		`INSERT INTO activities (issue_id, field, old_value, new_value) VALUES (?,?,?,?)`,
		issueID, field, oldV, newV)
}

// nullString returns a nullable bound for a possibly-empty date/value.
func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func ptrStr[T any](p *T) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%v", *p)
}
