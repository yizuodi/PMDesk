package web

import "net/http"

// handleBoard renders the kanban board with four status columns.
func (s *Server) handleBoard(w http.ResponseWriter, r *http.Request) {
	cols, err := s.store.BoardColumns()
	if err != nil {
		serverError(w, err)
		return
	}
	m := s.base(r, "board", "看板")
	m["Columns"] = cols
	m["Statuses"] = issueStatuses
	s.r.RenderPage(w, "issues/board", m)
}
