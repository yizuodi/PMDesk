package store

import (
	"database/sql"
	"time"
)

// Dashboard bundles the data needed to render the home page.
type Dashboard struct {
	CountsByStatus   map[string]int
	CountsByType     map[string]int
	CountsByPriority map[string]int
	Total            int
	Done             int
	InProgress       int
	Overdue          []Issue
	DueSoon          []Issue
	UpcomingReleases []Version
	RecentActivity   []Issue // recently updated issues
}

// GetDashboard assembles the dashboard view in a handful of queries.
func (s *Store) GetDashboard() (Dashboard, error) {
	d := Dashboard{
		CountsByStatus:   map[string]int{"todo": 0, "in_progress": 0, "in_review": 0, "done": 0},
		CountsByType:     map[string]int{"epic": 0, "story": 0, "task": 0, "bug": 0},
		CountsByPriority: map[string]int{"highest": 0, "high": 0, "medium": 0, "low": 0, "lowest": 0},
	}

	// Totals by status / type / priority in one scan.
	rows, err := s.DB.QueryContext(ctx(5*time.Second),
		`SELECT status, type, priority FROM issues`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var st, ty, pr string
		if err := rows.Scan(&st, &ty, &pr); err != nil {
			rows.Close()
			return d, err
		}
		d.CountsByStatus[st]++
		d.CountsByType[ty]++
		d.CountsByPriority[pr]++
		d.Total++
		if st == "done" {
			d.Done++
		}
		if st == "in_progress" {
			d.InProgress++
		}
	}
	rows.Close()

	var err2 error
	d.Overdue, err2 = s.ListIssues(IssueFilter{Overdue: true})
	if err2 != nil {
		return d, err2
	}
	d.DueSoon, err2 = s.ListIssues(IssueFilter{DueWithin: 3})
	if err2 != nil {
		return d, err2
	}
	// Limit overdue / due-soon to the most pressing items.
	if len(d.Overdue) > 20 {
		d.Overdue = d.Overdue[:20]
	}
	if len(d.DueSoon) > 20 {
		d.DueSoon = d.DueSoon[:20]
	}

	if d.UpcomingReleases, err2 = s.upcomingReleases(); err2 != nil {
		return d, err2
	}
	if d.RecentActivity, err2 = s.recentlyUpdated(8); err2 != nil {
		return d, err2
	}
	return d, nil
}

// upcomingReleases returns unreleased versions ordered by 转测日期 ascending,
// with undated versions last (the order provided by ListVersions).
func (s *Store) upcomingReleases() ([]Version, error) {
	all, err := s.ListVersions()
	if err != nil {
		return nil, err
	}
	var out []Version
	for _, v := range all {
		if v.Status != "released" {
			out = append(out, v)
		}
	}
	if len(out) > 5 {
		out = out[:5]
	}
	return out, nil
}

// recentlyUpdated returns the N most recently touched issues.
func (s *Store) recentlyUpdated(n int) ([]Issue, error) {
	rows, err := s.DB.QueryContext(ctx(5*time.Second),
		`SELECT i.id, i.seq, i.key, i.title, '', i.type, i.status, i.priority,
			i.due_date, i.board_order, i.created_at, i.updated_at, 0
		FROM issues i
		ORDER BY i.updated_at DESC, i.id DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
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
			ds := dueDate.String
			i.DueDate = &ds
		}
		out = append(out, i)
	}
	return out, rows.Err()
}
