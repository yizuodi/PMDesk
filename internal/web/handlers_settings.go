package web

import (
	"net/http"
	"strings"
)

// handleSettings renders the project settings form.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	p, _ := s.store.GetProject()
	m := s.base(r, "settings", "设置")
	m["P"] = p
	s.r.RenderPage(w, "settings", m)
}

// handleSettingsSave handles POST /settings.
func (s *Server) handleSettingsSave(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PostFormValue("name"))
	key := strings.TrimSpace(r.PostFormValue("key"))
	if name == "" {
		name = "My Project"
	}
	if key == "" {
		key = "PM"
	}
	// Keep the key short and alphanumeric-ish (used in issue keys like PM-123).
	key = sanitizeKey(key)
	if err := s.store.UpdateProject(name, key); err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/settings?msg="+q("设置已保存"))
}

// sanitizeKey uppercases and keeps A-Z0-9 only, capping length.
func sanitizeKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 32)
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
		if b.Len() >= 6 {
			break
		}
	}
	if b.Len() == 0 {
		return "PM"
	}
	return b.String()
}
