package web

import (
	"net/http"
	"strings"
)

// handleCommentAdd handles POST /issues/{id}/comments. Returns the rendered
// comment fragment so HTMX can prepend it to the comment list.
func (s *Server) handleCommentAdd(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	body := strings.TrimSpace(r.PostFormValue("body"))
	if body == "" {
		badRequest(w, "评论不能为空")
		return
	}
	c, err := s.store.AddComment(id, body)
	if err != nil {
		serverError(w, err)
		return
	}
	// Allow HTMX to target the new node via an out-of-band swap id.
	w.Header().Set("HX-Trigger", "commentAdded")
	s.r.RenderFragment(w, "comment", map[string]any{"C": c, "CanDelete": true})
}

// handleCommentDelete handles POST /comments/{id}/delete. HTMX removes the row
// client-side via hx-swap="delete" on the triggering element.
func (s *Server) handleCommentDelete(w http.ResponseWriter, r *http.Request) {
	cid := idParam(r)
	if err := s.store.DeleteComment(cid); err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
