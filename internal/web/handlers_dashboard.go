package web

import "net/http"

// handleDashboard renders the home page with stats and reminders.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	d, err := s.store.GetDashboard()
	if err != nil {
		serverError(w, err)
		return
	}
	m := s.base(r, "dashboard", "仪表盘")
	m["Dash"] = d
	m["OverdueCount"] = len(d.Overdue)
	s.r.RenderPage(w, "dashboard", m)
}
