package web

import (
	"html/template"
	"net/http"
	"strings"

	"pmjira/internal/store"
)

// handlePeopleList renders the 人员速查 page with the people table and the
// contact-links section.
func (s *Server) handlePeopleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	people, err := s.store.ListPeople(strings.TrimSpace(q.Get("q")), q.Get("team"), q.Get("role"))
	if err != nil {
		serverError(w, err)
		return
	}
	teams, _ := s.store.Teams()
	roles, _ := s.store.AllRoles()
	links, _ := s.store.ListContactLinks()

	m := s.base(r, "people", "人员速查")
	m["People"] = people
	m["Teams"] = teams
	m["AllRoles"] = roles
	m["Links"] = links
	m["FQ"] = q.Get("q")
	m["FTeam"] = q.Get("team")
	m["FRole"] = q.Get("role")
	s.r.RenderPage(w, "people/list", m)
}

func personInputFromForm(r *http.Request) store.PersonInput {
	return store.PersonInput{
		Name:       strings.TrimSpace(r.PostFormValue("name")),
		EmployeeID: strings.TrimSpace(r.PostFormValue("employee_id")),
		Roles:      r.PostFormValue("roles"),
		Team:       strings.TrimSpace(r.PostFormValue("team")),
	}
}

// handlePersonForm renders the create/edit person form.
func (s *Server) handlePersonForm(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	editing := id > 0
	var p store.Person
	if editing {
		pp, err := s.store.GetPerson(id)
		if err != nil {
			notFound(w)
			return
		}
		p = pp
	}
	m := s.base(r, "people", "人员")
	m["Person"] = p
	m["Editing"] = editing
	m["RolesJSON"] = rolesToJS(p.Roles)
	if msg := r.URL.Query().Get("msg"); msg != "" {
		m["Flash"] = msg
	}
	s.r.RenderPage(w, "people/form", m)
}

// rolesToJS renders a JS array literal of role strings for the tag editor init.
// Returned as template.JS so html/template emits it verbatim (a plain string
// would be quoted into a JS string literal, breaking the array).
func rolesToJS(roles []string) template.JS {
	var b strings.Builder
	b.WriteString("[")
	for i, r := range roles {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\"")
		b.WriteString(strings.ReplaceAll(strings.ReplaceAll(r, "\\", "\\\\"), "\"", "\\\""))
		b.WriteString("\"")
	}
	b.WriteString("]")
	return template.JS(b.String())
}

// handlePersonCreate handles POST /people.
func (s *Server) handlePersonCreate(w http.ResponseWriter, r *http.Request) {
	in := personInputFromForm(r)
	if in.Name == "" {
		http.Redirect(w, r, "/people/new?msg="+q("姓名不能为空"), http.StatusSeeOther)
		return
	}
	p, err := s.store.CreatePerson(in)
	if err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/people?msg="+q("已添加 "+p.Name))
}

// handlePersonUpdate handles POST /people/{id}.
func (s *Server) handlePersonUpdate(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	if id == 0 {
		notFound(w)
		return
	}
	in := personInputFromForm(r)
	if in.Name == "" {
		http.Redirect(w, r, "/people/"+itoa(id)+"/edit?msg="+q("姓名不能为空"), http.StatusSeeOther)
		return
	}
	if err := s.store.UpdatePerson(id, in); err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/people?msg="+q("已保存"))
}

// handlePersonDelete handles POST /people/{id}/delete.
func (s *Server) handlePersonDelete(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	if err := s.store.DeletePerson(id); err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/people?msg="+q("已删除"))
}

// ---- contact links ----

func contactInputFromForm(r *http.Request) store.ContactLinkInput {
	return store.ContactLinkInput{
		Name: strings.TrimSpace(r.PostFormValue("name")),
		URL:  strings.TrimSpace(r.PostFormValue("url")),
		Note: strings.TrimSpace(r.PostFormValue("note")),
	}
}

// handleContactCreate handles POST /people/contacts.
func (s *Server) handleContactCreate(w http.ResponseWriter, r *http.Request) {
	in := contactInputFromForm(r)
	if in.Name == "" || in.URL == "" {
		redirect(w, r, "/people?msg="+q("名称和网址不能为空"))
		return
	}
	if _, err := s.store.CreateContactLink(in); err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/people?msg="+q("已添加接口人链接"))
}

// handleContactUpdate handles POST /people/contacts/{id}/update.
func (s *Server) handleContactUpdate(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	in := contactInputFromForm(r)
	if in.Name == "" || in.URL == "" {
		redirect(w, r, "/people?msg="+q("名称和网址不能为空"))
		return
	}
	if err := s.store.UpdateContactLink(id, in); err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/people?msg="+q("已更新接口人链接"))
}

// handleContactDelete handles POST /people/contacts/{id}/delete.
func (s *Server) handleContactDelete(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	if err := s.store.DeleteContactLink(id); err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/people?msg="+q("已删除接口人链接"))
}

// handleContactMove handles POST /people/contacts/{id}/{dir} (up|down).
func (s *Server) handleContactMove(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	dir := -1
	if r.PathValue("dir") == "down" {
		dir = 1
	}
	if err := s.store.MoveContactLink(id, dir); err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/people")
}
