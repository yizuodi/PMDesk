package web

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"pmjira/internal/store"
)

// RGroup is one R-version bucket in the grouped version tree.
type RGroup struct {
	R         int
	CVersions []CGroup
}

// CGroup is one (R,C) bucket; since C versions are not separate entities, every
// version with a matching R/C (B builds and SPC/CP pre-releases) is listed here.
type CGroup struct {
	R, C  int
	Items []store.Version
}

// versionInputFromForm reads editable version fields from POST data.
func versionInputFromForm(r *http.Request) store.VersionInput {
	in := store.VersionInput{
		Name:        strings.TrimSpace(r.PostFormValue("name")),
		Description: r.PostFormValue("description"),
		StartDate:   r.PostFormValue("start_date"),
		ReleaseDate: r.PostFormValue("release_date"),
		Status:      r.PostFormValue("status"),
	}
	if in.Status == "" {
		in.Status = "planned"
	}
	return in
}

// handleVersionsList renders the version list in one of three views selected by
// ?view=: grouped (default, R→C tree), flat (filterable list), roadmap (timeline).
func (s *Server) handleVersionsList(w http.ResponseWriter, r *http.Request) {
	all, err := s.store.ListVersions()
	if err != nil {
		serverError(w, err)
		return
	}
	switch r.URL.Query().Get("view") {
	case "flat":
		s.renderVersionsFlat(w, r, all)
	case "roadmap":
		s.renderVersionsRoadmapView(w, r, all)
	default:
		s.renderVersionsGrouped(w, r, all)
	}
}

// renderVersionsRoadmapView renders the roadmap timeline within the versions page.
func (s *Server) renderVersionsRoadmapView(w http.ResponseWriter, r *http.Request, all []store.Version) {
	m := s.base(r, "versions", "版本计划 · 路线图")
	m["Rows"] = all
	m["View"] = "roadmap"
	s.r.RenderPage(w, "versions/roadmap", m)
}

// renderVersionsFlat applies query filters and renders the flat list.
func (s *Server) renderVersionsFlat(w http.ResponseWriter, r *http.Request, all []store.Version) {
	q := r.URL.Query()
	fR := q.Get("r")
	fC := q.Get("c")
	fType := q.Get("type")
	fStatus := q.Get("status")

	// Distinct R / C values present, for the filter dropdowns.
	rSet := map[int]bool{}
	cSet := map[int]bool{}
	for _, v := range all {
		if v.RNum != nil {
			rSet[*v.RNum] = true
		}
		if v.CNum != nil {
			cSet[*v.CNum] = true
		}
	}
	rs := toSortedInts(rSet)
	cs := toSortedInts(cSet)

	var versions []store.Version
	for _, v := range all {
		if fR != "" && (v.RNum == nil || strconv.Itoa(*v.RNum) != fR) {
			continue
		}
		if fC != "" && (v.CNum == nil || strconv.Itoa(*v.CNum) != fC) {
			continue
		}
		if fType != "" && v.VType != fType {
			continue
		}
		if fStatus != "" && v.Status != fStatus {
			continue
		}
		versions = append(versions, v)
	}

	m := s.base(r, "versions", "版本计划")
	m["Versions"] = versions
	m["Rs"] = rs
	m["Cs"] = cs
	m["FR"] = fR
	m["FC"] = fC
	m["FType"] = fType
	m["FStatus"] = fStatus
	m["View"] = "flat"
	s.r.RenderPage(w, "versions/list", m)
}

// renderVersionsGrouped builds the R → C tree and renders the grouped view.
// Every version carrying R/C components is placed under its (R, C) bucket;
// C versions are not separate entities, so a bucket's header is just the
// derived "R###C##" label and all matching versions are listed as items.
func (s *Server) renderVersionsGrouped(w http.ResponseWriter, r *http.Request, all []store.Version) {
	type bucket struct {
		items []store.Version
	}
	buckets := map[[2]int]*bucket{}
	order := [][2]int{}
	var legacy []store.Version

	for _, v := range all {
		if v.RNum == nil || v.CNum == nil {
			legacy = append(legacy, v)
			continue
		}
		key := [2]int{*v.RNum, *v.CNum}
		b, ok := buckets[key]
		if !ok {
			b = &bucket{}
			buckets[key] = b
			order = append(order, key)
		}
		b.items = append(b.items, v)
	}

	byR := map[int]*RGroup{}
	rOrder := []int{}
	for _, key := range order {
		r, c := key[0], key[1]
		rg, ok := byR[r]
		if !ok {
			rg = &RGroup{R: r}
			byR[r] = rg
			rOrder = append(rOrder, r)
		}
		items := buckets[key].items
		sortVersionsByDate(items)
		rg.CVersions = append(rg.CVersions, CGroup{R: r, C: c, Items: items})
	}
	sort.Sort(sort.Reverse(sort.IntSlice(rOrder)))
	// C buckets within an R group, descending by C.
	for _, rg := range byR {
		sort.Slice(rg.CVersions, func(i, j int) bool { return rg.CVersions[i].C > rg.CVersions[j].C })
	}

	var groups []RGroup
	for _, r := range rOrder {
		groups = append(groups, *byR[r])
	}

	m := s.base(r, "versions", "版本计划 · 按版本分组")
	m["Groups"] = groups
	m["Legacy"] = legacy
	m["View"] = "group"
	s.r.RenderPage(w, "versions/group", m)
}

// sortVersionsByDate orders items by 转测日期 (start_date) ascending, undated last.
func sortVersionsByDate(items []store.Version) {
	hasDate := func(v store.Version) (bool, string) {
		if v.StartDate == nil || *v.StartDate == "" {
			return false, ""
		}
		return true, *v.StartDate
	}
	sort.SliceStable(items, func(i, j int) bool {
		hi, di := hasDate(items[i])
		hj, dj := hasDate(items[j])
		if hi != hj {
			return hi // dated before undated
		}
		if !hi {
			return items[i].Name < items[j].Name
		}
		return di < dj
	})
}

func toSortedInts(set map[int]bool) []int {
	out := make([]int, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out)))
	return out
}

// handleVersionForm renders the create/edit version form.
func (s *Server) handleVersionForm(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	editing := id > 0
	var v store.Version
	if editing {
		vv, err := s.store.GetVersion(id)
		if err != nil {
			notFound(w)
			return
		}
		v = vv
	}
	m := s.base(r, "versions", "版本")
	m["V"] = v
	m["Editing"] = editing
	if msg := r.URL.Query().Get("msg"); msg != "" {
		m["Flash"] = msg
	}
	s.r.RenderPage(w, "versions/form", m)
}

// handleVersionCreate handles POST /versions.
func (s *Server) handleVersionCreate(w http.ResponseWriter, r *http.Request) {
	in := versionInputFromForm(r)
	if in.Name == "" {
		http.Redirect(w, r, "/versions/new?msg="+q("名称不能为空"), http.StatusSeeOther)
		return
	}
	v, err := s.store.CreateVersion(in)
	if err != nil {
		http.Redirect(w, r, "/versions/new?msg="+q(err.Error()), http.StatusSeeOther)
		return
	}
	redirect(w, r, "/versions/"+itoa(v.ID)+"?msg="+q("已创建版本"))
}

// handleVersionDetail renders a standalone version (no linked issues).
func (s *Server) handleVersionDetail(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	v, err := s.store.GetVersion(id)
	if err != nil {
		notFound(w)
		return
	}
	m := s.base(r, "versions", v.Name)
	m["V"] = v
	s.r.RenderPage(w, "versions/detail", m)
}

// handleVersionUpdate handles POST /versions/{id}.
func (s *Server) handleVersionUpdate(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	if id == 0 {
		notFound(w)
		return
	}
	in := versionInputFromForm(r)
	if in.Name == "" {
		http.Redirect(w, r, "/versions/"+itoa(id)+"/edit?msg="+q("名称不能为空"), http.StatusSeeOther)
		return
	}
	if err := s.store.UpdateVersion(id, in); err != nil {
		http.Redirect(w, r, "/versions/"+itoa(id)+"/edit?msg="+q(err.Error()), http.StatusSeeOther)
		return
	}
	redirect(w, r, "/versions/"+itoa(id)+"?msg="+q("已保存"))
}

// handleVersionDelete handles POST /versions/{id}/delete.
func (s *Server) handleVersionDelete(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	if err := s.store.DeleteVersion(id); err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/versions?msg="+q("已删除版本"))
}
