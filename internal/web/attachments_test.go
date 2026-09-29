package web

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"pmjira/internal/store"
)

func multipartRequest(t *testing.T, method, target, filename string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, target, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestAttachmentUploadAndDeleteCleansFile(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "pmjira.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := New(st, NewConfigProvider(filepath.Join(dir, "jira_config.json")))
	issue, err := st.CreateIssue(store.IssueInput{Title: "带附件", Type: "task", Status: "todo", Priority: "medium"})
	if err != nil {
		t.Fatal(err)
	}

	content := []byte("attachment body")
	req := multipartRequest(t, http.MethodPost, "/issues/"+itoa(issue.ID)+"/attachments", "../report.txt", content)
	req.SetPathValue("id", itoa(issue.ID))
	w := httptest.NewRecorder()
	srv.handleAttachmentUpload(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", w.Code, w.Body.String())
	}
	attachments, err := st.ListAttachments(issue.ID)
	if err != nil || len(attachments) != 1 {
		t.Fatalf("attachments = %#v, err=%v", attachments, err)
	}
	attachment := attachments[0]
	path := st.AttachmentPath(attachment.Stored)
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, content) {
		t.Fatalf("stored attachment = %q, err=%v", got, err)
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/attachments/"+itoa(attachment.ID)+"/delete", nil)
	deleteReq.SetPathValue("id", itoa(attachment.ID))
	deleteW := httptest.NewRecorder()
	srv.handleAttachmentDelete(deleteW, deleteReq)
	if deleteW.Code != http.StatusSeeOther {
		t.Fatalf("delete status = %d", deleteW.Code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("attachment file still exists: %v", err)
	}
}

func TestSanitizeFilename(t *testing.T) {
	if got := sanitizeFilename(`../evil/report.txt`); got != "report.txt" {
		t.Fatalf("sanitizeFilename = %q", got)
	}
}
