package web

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"pmjira/internal/store"
)

var issueStatuses = []string{"todo", "in_progress", "in_review", "done"}

// issueFilterFromQuery builds an IssueFilter from query string parameters.
func issueFilterFromQuery(r *http.Request) store.IssueFilter {
	q := r.URL.Query()
	return store.IssueFilter{
		Search:   strings.TrimSpace(q.Get("q")),
		Type:     q.Get("type"),
		Status:   q.Get("status"),
		Priority: q.Get("priority"),
	}
}

// handleIssuesList renders the filterable issue table.
func (s *Server) handleIssuesList(w http.ResponseWriter, r *http.Request) {
	f := issueFilterFromQuery(r)
	issues, err := s.store.ListIssues(f)
	if err != nil {
		serverError(w, err)
		return
	}

	m := s.base(r, "issues", "个人事务")
	m["Issues"] = issues
	m["Filter"] = f
	s.r.RenderPage(w, "issues/list", m)
}

// handleIssueForm renders the create/edit form.
func (s *Server) handleIssueForm(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	editing := id > 0
	var issue store.Issue
	if editing {
		iss, err := s.store.GetIssue(id)
		if err != nil {
			notFound(w)
			return
		}
		issue = iss
	}

	// Current field values, defaulting to sensible values for new issues.
	cur := map[string]any{
		"type": "task", "status": "todo", "priority": "medium", "due_date": "",
	}
	if editing {
		cur["type"] = issue.Type
		cur["status"] = issue.Status
		cur["priority"] = issue.Priority
		cur["due_date"] = strPtrVal(issue.DueDate)
	}

	m := s.base(r, "issues", "事务")
	m["Issue"] = issue
	m["Editing"] = editing
	m["Cur"] = cur
	if msg := r.URL.Query().Get("msg"); msg != "" {
		m["Flash"] = msg
	}
	s.r.RenderPage(w, "issues/form", m)
}

func strPtrVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// issueInputFromForm reads editable issue fields from POST data.
func issueInputFromForm(r *http.Request) store.IssueInput {
	in := store.IssueInput{
		Title:       strings.TrimSpace(r.PostFormValue("title")),
		Description: r.PostFormValue("description"),
		Type:        r.PostFormValue("type"),
		Status:      r.PostFormValue("status"),
		Priority:    r.PostFormValue("priority"),
		DueDate:     r.PostFormValue("due_date"),
	}
	if in.Type == "" {
		in.Type = "task"
	}
	if in.Status == "" {
		in.Status = "todo"
	}
	if in.Priority == "" {
		in.Priority = "medium"
	}
	return in
}

// handleIssueCreate handles POST /issues.
func (s *Server) handleIssueCreate(w http.ResponseWriter, r *http.Request) {
	in := issueInputFromForm(r)
	if in.Title == "" {
		http.Redirect(w, r, "/issues/new?msg="+q("标题不能为空"), http.StatusSeeOther)
		return
	}
	issue, err := s.store.CreateIssue(in)
	if err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/issues/"+itoa(issue.ID)+"?msg="+q("已创建 "+issue.Key))
}

// handleIssueDetail renders a single issue with comments and history.
func (s *Server) handleIssueDetail(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	issue, err := s.store.GetIssue(id)
	if err != nil {
		notFound(w)
		return
	}
	comments, _ := s.store.ListComments(id)
	activities, _ := s.store.ListActivities(id)
	attachments, _ := s.store.ListAttachments(id)

	m := s.base(r, "issues", issue.Key)
	m["Issue"] = issue
	m["Comments"] = comments
	m["Activities"] = activities
	m["Attachments"] = attachments
	m["Statuses"] = issueStatuses
	s.r.RenderPage(w, "issues/detail", m)
}

// handleIssueUpdate handles POST /issues/{id}.
func (s *Server) handleIssueUpdate(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	if id == 0 {
		notFound(w)
		return
	}
	in := issueInputFromForm(r)
	if in.Title == "" {
		http.Redirect(w, r, "/issues/"+itoa(id)+"/edit?msg="+q("标题不能为空"), http.StatusSeeOther)
		return
	}
	if _, err := s.store.UpdateIssue(id, in); err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/issues/"+itoa(id)+"?msg="+q("已保存"))
}

// handleIssueDelete handles POST /issues/{id}/delete. Attachment files on disk
// are removed before the row (and its cascaded attachment rows) go away.
func (s *Server) handleIssueDelete(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	if files, _ := s.store.ListAttachmentFiles(id); files != nil {
		for _, f := range files {
			_ = os.Remove(s.store.AttachmentPath(f))
		}
	}
	if err := s.store.DeleteIssue(id); err != nil {
		serverError(w, err)
		return
	}
	redirect(w, r, "/issues?msg="+q("已删除"))
}

// handleIssueStatus handles a status change from either the detail page (plain
// form POST → redirect back) or the board (HTMX request → returns the refreshed
// board grid fragment).
func (s *Server) handleIssueStatus(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	status := r.PostFormValue("status")
	if !validStatus(status) {
		badRequest(w, "invalid status")
		return
	}
	if err := s.store.SetIssueStatus(id, status); err != nil {
		serverError(w, err)
		return
	}
	if r.Header.Get("HX-Request") == "true" {
		// Board drag-drop: return only the board grid for an in-place swap.
		cols, err := s.store.BoardColumns()
		if err != nil {
			serverError(w, err)
			return
		}
		m := map[string]any{"Columns": cols, "Statuses": issueStatuses, "IsAdmin": true}
		s.r.RenderFragment(w, "board_grid", m)
		return
	}
	// Detail-page quick change: redirect back to the issue.
	redirect(w, r, "/issues/"+itoa(id))
}

func validStatus(s string) bool {
	for _, v := range issueStatuses {
		if v == s {
			return true
		}
	}
	return false
}

func itoa(n int64) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	return string(buf[i:])
}

// q encodes a string for use in a URL query value (e.g. CJK flash messages).
func q(s string) string { return url.QueryEscape(s) }
