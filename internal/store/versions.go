package store

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Version type identifiers for the structured version scheme.
const (
	VTypeC     = "C"   // R###C##      — half-year container
	VTypeB     = "B"   // R###C##B###  — internal dev build
	VTypeSPC   = "SPC" // R###C##SPC###B###  — pre-release
	VTypeCP    = "CP"  // R###C##CP####B###  — pre-release
	VTypeKey   = "KEY" // R###C##<suffix>     — key milestone (e.g. R27C10迭代0, R27C00TR5)
	VTypeOther = ""    // legacy / non-conforming
)

var (
	reSPC = regexp.MustCompile(`(?i)^R(\d+)C(\d+)SPC(\d+)B(\d+)$`) // R027C00SPC200B020
	reCP  = regexp.MustCompile(`(?i)^R(\d+)C(\d+)CP(\d+)B(\d+)$`)  // R027C00CP1022B010
	reRCB = regexp.MustCompile(`(?i)^R(\d+)C(\d+)B(\d+)$`)         // R027C10B003
	reRC  = regexp.MustCompile(`(?i)^R(\d+)C(\d+)$`)               // R027C10
	// Key milestone: R##C## followed by a non-empty, non-digit-led suffix
	// (so it doesn't shadow B/SPC/CP). Examples: R27C10迭代0, R27C00TR5.
	reKey = regexp.MustCompile(`(?i)^R(\d+)C(\d+)([A-Za-z一-龥].*)$`)
)

// ParsedVersion is the structured decomposition of a version string.
type ParsedVersion struct {
	Type  string // C | B | SPC | CP | "" (other)
	R     int
	C     int
	B     int
	Extra int // SPC/CP sequence number (the 200 / 1022 part)
}

// OK reports whether the string matched a known scheme.
func (p ParsedVersion) OK() bool { return p.Type != VTypeOther }

// ParseVersion decomposes a version string into its structured components.
// Matching is case-insensitive; callers should store the canonical upper form.
// Note: for KEY milestones the suffix case is preserved (e.g. 迭代0 / TR5).
func ParseVersion(s string) ParsedVersion {
	s = strings.TrimSpace(s)
	if m := reSPC.FindStringSubmatch(s); m != nil {
		return ParsedVersion{Type: VTypeSPC, R: atoiSafe(m[1]), C: atoiSafe(m[2]), Extra: atoiSafe(m[3]), B: atoiSafe(m[4])}
	}
	if m := reCP.FindStringSubmatch(s); m != nil {
		return ParsedVersion{Type: VTypeCP, R: atoiSafe(m[1]), C: atoiSafe(m[2]), Extra: atoiSafe(m[3]), B: atoiSafe(m[4])}
	}
	if m := reRCB.FindStringSubmatch(s); m != nil {
		return ParsedVersion{Type: VTypeB, R: atoiSafe(m[1]), C: atoiSafe(m[2]), B: atoiSafe(m[3])}
	}
	if m := reKey.FindStringSubmatch(s); m != nil {
		return ParsedVersion{Type: VTypeKey, R: atoiSafe(m[1]), C: atoiSafe(m[2])} // suffix kept in Name
	}
	if m := reRC.FindStringSubmatch(s); m != nil {
		return ParsedVersion{Type: VTypeC, R: atoiSafe(m[1]), C: atoiSafe(m[2])}
	}
	return ParsedVersion{}
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// Version is a release / planned iteration.
type Version struct {
	ID          int64
	Name        string
	Description string
	StartDate   *string
	ReleaseDate *string
	Status      string
	CreatedAt   string
	UpdatedAt   string

	// Structured version scheme fields (nullable; derived from Name on save).
	VType    string
	RNum     *int
	CNum     *int
	BNum     *int
	ExtraNum *int
}

func ptrInt(n int) *int { return &n }

// versionColsBase is the canonical column list for plain version reads.
const versionColsBase = `id, name, description, start_date, release_date, status,
	created_at, updated_at, vtype, r_num, c_num, b_num, extra_num`

func scanVersion(row interface {
	Scan(dest ...any) error
}) (Version, error) {
	var v Version
	var start, release sql.NullString
	var rnum, cnum, bnum, extra sql.NullInt64
	var vtype sql.NullString
	err := row.Scan(&v.ID, &v.Name, &v.Description, &start, &release, &v.Status,
		&v.CreatedAt, &v.UpdatedAt, &vtype, &rnum, &cnum, &bnum, &extra)
	if err != nil {
		return Version{}, err
	}
	if vtype.Valid {
		v.VType = vtype.String
	}
	if start.Valid {
		s := start.String
		v.StartDate = &s
	}
	if release.Valid {
		s := release.String
		v.ReleaseDate = &s
	}
	if rnum.Valid {
		n := int(rnum.Int64)
		v.RNum = &n
	}
	if cnum.Valid {
		n := int(cnum.Int64)
		v.CNum = &n
	}
	if bnum.Valid {
		n := int(bnum.Int64)
		v.BNum = &n
	}
	if extra.Valid {
		n := int(extra.Int64)
		v.ExtraNum = &n
	}
	return v, nil
}

// listQueryBase returns the SELECT .. FROM shared by version reads.
func listQueryBase() string {
	return `SELECT v.id, v.name, v.description, v.start_date, v.release_date, v.status,
			v.created_at, v.updated_at, v.vtype, v.r_num, v.c_num, v.b_num, v.extra_num
		FROM versions v `
}

// ListVersions returns all versions ordered by 转测日期 (start_date) ascending,
// with undated versions last.
func (s *Store) ListVersions() ([]Version, error) {
	rows, err := s.DB.QueryContext(ctx(5*time.Second),
		listQueryBase()+`ORDER BY v.start_date IS NULL, v.start_date ASC, v.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetVersion returns one version by id.
func (s *Store) GetVersion(id int64) (Version, error) {
	row := s.DB.QueryRowContext(ctx(5*time.Second), listQueryBase()+`WHERE v.id = ?`, id)
	v, err := scanVersion(row)
	if err != nil {
		return Version{}, fmt.Errorf("get version: %w", err)
	}
	return v, nil
}

// VersionInput holds editable version fields.
type VersionInput struct {
	Name        string
	Description string
	StartDate   string
	ReleaseDate string
	Status      string
}

// CreateVersion inserts a new version, parsing the name into structured fields.
func (s *Store) CreateVersion(in VersionInput) (Version, error) {
	cols, err := s.resolveVersion(in)
	if err != nil {
		return Version{}, err
	}
	res, err := s.DB.ExecContext(ctx(5*time.Second), `INSERT INTO versions
		(name, description, start_date, release_date, status, vtype, r_num, c_num, b_num, extra_num)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		cols.name, in.Description, nullString(in.StartDate), nullString(in.ReleaseDate), in.Status,
		cols.vtype, cols.r, cols.c, cols.b, cols.extra)
	if err != nil {
		return Version{}, fmt.Errorf("create version: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetVersion(id)
}

// UpdateVersion replaces a version's editable fields, re-parsing the name.
func (s *Store) UpdateVersion(id int64, in VersionInput) error {
	cols, err := s.resolveVersion(in)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx(5*time.Second), `UPDATE versions SET
		name=?, description=?, start_date=?, release_date=?, status=?,
		vtype=?, r_num=?, c_num=?, b_num=?, extra_num=?,
		updated_at=strftime('%Y-%m-%dT%H:%M:%SZ','now') WHERE id=?`,
		cols.name, in.Description, nullString(in.StartDate), nullString(in.ReleaseDate), in.Status,
		cols.vtype, cols.r, cols.c, cols.b, cols.extra, id)
	return err
}

// DeleteVersion removes a version.
func (s *Store) DeleteVersion(id int64) error {
	_, err := s.DB.ExecContext(ctx(5*time.Second), `DELETE FROM versions WHERE id=?`, id)
	return err
}

// versionCols holds the resolved, DB-ready values for a version's structured columns.
type versionCols struct {
	name                  string
	vtype, r, c, b, extra any
}

// resolveVersion parses the name (upper-cased for scheme versions) and computes
// the structured column values. SPC/CP carry their own R/C in the string, so no
// parent link is needed.
func (s *Store) resolveVersion(in VersionInput) (versionCols, error) {
	name := strings.TrimSpace(in.Name)
	pv := ParseVersion(name)
	// Upper-case only the structured scheme forms; KEY milestones keep their
	// original suffix case (e.g. 迭代0, TR5).
	if pv.OK() && pv.Type != VTypeKey {
		name = strings.ToUpper(name)
	}
	out := versionCols{name: name, vtype: pv.Type}
	switch pv.Type {
	case VTypeC:
		out.r, out.c = pv.R, pv.C
	case VTypeB:
		out.r, out.c, out.b = pv.R, pv.C, pv.B
	case VTypeSPC, VTypeCP:
		out.r, out.c, out.b, out.extra = pv.R, pv.C, pv.B, pv.Extra
	case VTypeKey:
		out.r, out.c = pv.R, pv.C
	case VTypeOther:
		// Non-conforming: keep vtype as "" (the column is NOT NULL DEFAULT '').
		out.vtype = VTypeOther
	}
	return out, nil
}
