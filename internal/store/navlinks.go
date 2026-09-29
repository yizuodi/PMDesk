package store

import (
	"fmt"
	"time"
)

// NavLink is a user-defined quick-navigation entry shown in the top bar.
type NavLink struct {
	ID        int64
	Title     string
	URL       string
	Mode      string // iframe | newtab
	Category  string // grouping label; "" = shown directly in the top bar
	Position  int
	CreatedAt string
}

// NavLinkInput holds editable nav-link fields.
type NavLinkInput struct {
	Title    string
	URL      string
	Mode     string
	Category string
}

// ListNavLinks returns all nav links ordered by category, then position, then id.
func (s *Store) ListNavLinks() ([]NavLink, error) {
	rows, err := s.DB.QueryContext(ctx(5*time.Second),
		`SELECT id, title, url, mode, category, position, created_at
		 FROM nav_links ORDER BY category ASC, position ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list nav links: %w", err)
	}
	defer rows.Close()
	var out []NavLink
	for rows.Next() {
		var n NavLink
		if err := rows.Scan(&n.ID, &n.Title, &n.URL, &n.Mode, &n.Category, &n.Position, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// GetNavLink returns one nav link by id.
func (s *Store) GetNavLink(id int64) (NavLink, error) {
	var n NavLink
	err := s.DB.QueryRowContext(ctx(5*time.Second),
		`SELECT id, title, url, mode, category, position, created_at FROM nav_links WHERE id=?`, id).
		Scan(&n.ID, &n.Title, &n.URL, &n.Mode, &n.Category, &n.Position, &n.CreatedAt)
	if err != nil {
		return NavLink{}, fmt.Errorf("get nav link: %w", err)
	}
	return n, nil
}

// CreateNavLink inserts a new nav link at the end of the list.
func (s *Store) CreateNavLink(in NavLinkInput) (NavLink, error) {
	mode := in.Mode
	if mode != "iframe" && mode != "newtab" {
		mode = "newtab"
	}
	res, err := s.DB.ExecContext(ctx(5*time.Second),
		`INSERT INTO nav_links (title, url, mode, category, position)
		 VALUES (?, ?, ?, ?, COALESCE((SELECT MAX(position) FROM nav_links), -1) + 1)`,
		in.Title, in.URL, mode, in.Category)
	if err != nil {
		return NavLink{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetNavLink(id)
}

// UpdateNavLink replaces a nav link's editable fields.
func (s *Store) UpdateNavLink(id int64, in NavLinkInput) error {
	mode := in.Mode
	if mode != "iframe" && mode != "newtab" {
		mode = "newtab"
	}
	_, err := s.DB.ExecContext(ctx(5*time.Second),
		`UPDATE nav_links SET title=?, url=?, mode=?, category=? WHERE id=?`,
		in.Title, in.URL, mode, in.Category, id)
	return err
}

// DeleteNavLink removes a nav link.
func (s *Store) DeleteNavLink(id int64) error {
	_, err := s.DB.ExecContext(ctx(5*time.Second), `DELETE FROM nav_links WHERE id=?`, id)
	return err
}

// MoveNavLink shifts a link's position up (dir=-1) or down (dir=+1) by swapping
// with its neighbour in the ordered list.
func (s *Store) MoveNavLink(id int64, dir int) error {
	links, err := s.ListNavLinks()
	if err != nil {
		return err
	}
	idx := -1
	for i, l := range links {
		if l.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	j := idx + dir
	if j < 0 || j >= len(links) {
		return nil
	}
	a, b := links[idx], links[j]
	tx, err := s.DB.BeginTx(ctx(5*time.Second), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE nav_links SET position=? WHERE id=?`, b.Position, a.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE nav_links SET position=? WHERE id=?`, a.Position, b.ID); err != nil {
		return err
	}
	return tx.Commit()
}
