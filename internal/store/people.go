package store

import (
	"fmt"
	"strings"
	"time"
)

// Person is a 人员速查 entry.
type Person struct {
	ID         int64
	Name       string
	EmployeeID string
	Roles      []string // parsed from comma-separated storage
	Team       string
	CreatedAt  string
	UpdatedAt  string
}

// ContactLink is an entry in the 接口人列表.
type ContactLink struct {
	ID        int64
	Name      string
	URL       string
	Note      string
	Position  int
	CreatedAt string
}

// PersonInput holds editable person fields. Roles is the comma-separated string
// from the form (the tag editor syncs into a hidden input).
type PersonInput struct {
	Name       string
	EmployeeID string
	Roles      string // comma-separated
	Team       string
}

// ContactLinkInput holds editable contact-link fields.
type ContactLinkInput struct {
	Name string
	URL  string
	Note string
}

// splitRoles splits a comma-separated role string into trimmed, non-empty tags.
func splitRoles(s string) []string {
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ListPeople returns all people, optionally filtered by team / role / search.
func (s *Store) ListPeople(search, team, role string) ([]Person, error) {
	q := `SELECT id, name, employee_id, roles, team, created_at, updated_at FROM people`
	var clauses []string
	var args []any
	if search != "" {
		clauses = append(clauses, "(name LIKE ? OR employee_id LIKE ? OR team LIKE ?)")
		args = append(args, "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if team != "" {
		clauses = append(clauses, "team = ?")
		args = append(args, team)
	}
	if role != "" {
		// roles is comma-separated; match as a whole tag on either side.
		clauses = append(clauses, "(',' || roles || ',') LIKE ?")
		args = append(args, "%,"+role+",%")
	}
	if len(clauses) > 0 {
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	q += " ORDER BY name ASC"
	rows, err := s.DB.QueryContext(ctx(5*time.Second), q, args...)
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}
	defer rows.Close()
	var out []Person
	for rows.Next() {
		var p Person
		var roles string
		if err := rows.Scan(&p.ID, &p.Name, &p.EmployeeID, &roles, &p.Team, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Roles = splitRoles(roles)
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPerson returns one person by id.
func (s *Store) GetPerson(id int64) (Person, error) {
	var p Person
	var roles string
	err := s.DB.QueryRowContext(ctx(5*time.Second),
		`SELECT id, name, employee_id, roles, team, created_at, updated_at FROM people WHERE id=?`, id).
		Scan(&p.ID, &p.Name, &p.EmployeeID, &roles, &p.Team, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return Person{}, fmt.Errorf("get person: %w", err)
	}
	p.Roles = splitRoles(roles)
	return p, nil
}

// CreatePerson inserts a new person.
func (s *Store) CreatePerson(in PersonInput) (Person, error) {
	res, err := s.DB.ExecContext(ctx(5*time.Second),
		`INSERT INTO people (name, employee_id, roles, team) VALUES (?,?,?,?)`,
		in.Name, in.EmployeeID, in.Roles, in.Team)
	if err != nil {
		return Person{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetPerson(id)
}

// UpdatePerson replaces a person's editable fields.
func (s *Store) UpdatePerson(id int64, in PersonInput) error {
	_, err := s.DB.ExecContext(ctx(5*time.Second),
		`UPDATE people SET name=?, employee_id=?, roles=?, team=?,
		 updated_at=strftime('%Y-%m-%dT%H:%M:%SZ','now') WHERE id=?`,
		in.Name, in.EmployeeID, in.Roles, in.Team, id)
	return err
}

// DeletePerson removes a person.
func (s *Store) DeletePerson(id int64) error {
	_, err := s.DB.ExecContext(ctx(5*time.Second), `DELETE FROM people WHERE id=?`, id)
	return err
}

// Teams returns distinct, non-empty team values for the filter dropdown.
func (s *Store) Teams() ([]string, error) {
	rows, err := s.DB.QueryContext(ctx(5*time.Second),
		`SELECT DISTINCT team FROM people WHERE team <> '' ORDER BY team`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// AllRoles returns distinct role tags across all people, for the filter dropdown.
func (s *Store) AllRoles() ([]string, error) {
	rows, err := s.DB.QueryContext(ctx(5*time.Second), `SELECT roles FROM people WHERE roles <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var out []string
	for rows.Next() {
		var roles string
		if err := rows.Scan(&roles); err != nil {
			return nil, err
		}
		for _, r := range splitRoles(roles) {
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	return out, rows.Err()
}

// ---- contact links ----

// ListContactLinks returns all contact links ordered by position.
func (s *Store) ListContactLinks() ([]ContactLink, error) {
	rows, err := s.DB.QueryContext(ctx(5*time.Second),
		`SELECT id, name, url, note, position, created_at FROM contact_links ORDER BY position ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list contact links: %w", err)
	}
	defer rows.Close()
	var out []ContactLink
	for rows.Next() {
		var c ContactLink
		if err := rows.Scan(&c.ID, &c.Name, &c.URL, &c.Note, &c.Position, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateContactLink inserts a new contact link at the end of the list.
func (s *Store) CreateContactLink(in ContactLinkInput) (ContactLink, error) {
	res, err := s.DB.ExecContext(ctx(5*time.Second),
		`INSERT INTO contact_links (name, url, note, position)
		 VALUES (?, ?, ?, COALESCE((SELECT MAX(position) FROM contact_links), -1) + 1)`,
		in.Name, in.URL, in.Note)
	if err != nil {
		return ContactLink{}, err
	}
	id, _ := res.LastInsertId()
	row := s.DB.QueryRowContext(ctx(5*time.Second),
		`SELECT id, name, url, note, position, created_at FROM contact_links WHERE id=?`, id)
	var c ContactLink
	if err := row.Scan(&c.ID, &c.Name, &c.URL, &c.Note, &c.Position, &c.CreatedAt); err != nil {
		return ContactLink{}, err
	}
	return c, nil
}

// UpdateContactLink replaces a contact link's editable fields.
func (s *Store) UpdateContactLink(id int64, in ContactLinkInput) error {
	_, err := s.DB.ExecContext(ctx(5*time.Second),
		`UPDATE contact_links SET name=?, url=?, note=? WHERE id=?`,
		in.Name, in.URL, in.Note, id)
	return err
}

// DeleteContactLink removes a contact link.
func (s *Store) DeleteContactLink(id int64) error {
	_, err := s.DB.ExecContext(ctx(5*time.Second), `DELETE FROM contact_links WHERE id=?`, id)
	return err
}

// MoveContactLink shifts a link's position up (dir=-1) or down (dir=+1).
func (s *Store) MoveContactLink(id int64, dir int) error {
	links, err := s.ListContactLinks()
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
	if _, err := tx.Exec(`UPDATE contact_links SET position=? WHERE id=?`, b.Position, a.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE contact_links SET position=? WHERE id=?`, a.Position, b.ID); err != nil {
		return err
	}
	return tx.Commit()
}
