package web

import (
	"net/http"
	"strings"

	"pmjira/internal/store"
)

// navModes are the supported link open behaviours.
var navModes = []string{"newtab", "iframe"}

// handleNavEmbed renders a full-page iframe embedding the link's URL.
func (s *Server) handleNavEmbed(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	n, err := s.store.GetNavLink(id)
	if err != nil {
		notFound(w)
		return
	}
	m := s.base(r, "nav"+itoa(id), n.Title)
	m["Link"] = n
	s.r.RenderPage(w, "nav_iframe", m)
}

// navInputFromForm reads nav-link fields from POST data.
func navInputFromForm(r *http.Request) store.NavLinkInput {
	mode := r.PostFormValue("mode")
	if mode == "" {
		mode = "newtab"
	}
	return store.NavLinkInput{
		Title:    strings.TrimSpace(r.PostFormValue("title")),
		URL:      strings.TrimSpace(r.PostFormValue("url")),
		Mode:     mode,
		Category: strings.TrimSpace(r.PostFormValue("category")),
	}
}

// handleNavCreate handles POST /settings/nav.
func (s *Server) handleNavCreate(w http.ResponseWriter, r *http.Request) {
	in := navInputFromForm(r)
	if in.Title == "" || in.URL == "" {
		redirect(w, r, "/settings?msg="+q("标题和网址不能为空"))
		return
	}
	if _, err := s.store.CreateNavLink(in); err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/settings?msg="+q("已添加导航"))
}

// handleNavUpdate handles POST /settings/nav/{id}/update.
func (s *Server) handleNavUpdate(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	in := navInputFromForm(r)
	if in.Title == "" || in.URL == "" {
		redirect(w, r, "/settings?msg="+q("标题和网址不能为空"))
		return
	}
	if err := s.store.UpdateNavLink(id, in); err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/settings?msg="+q("已更新导航"))
}

// handleNavDelete handles POST /settings/nav/{id}/delete.
func (s *Server) handleNavDelete(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	if err := s.store.DeleteNavLink(id); err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/settings?msg="+q("已删除导航"))
}

// handleNavMove handles POST /settings/nav/{id}/{dir} where dir is "up" or "down".
func (s *Server) handleNavMove(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	dir := -1
	if r.PathValue("dir") == "down" {
		dir = 1
	}
	if err := s.store.MoveNavLink(id, dir); err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/settings")
}
