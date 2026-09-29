package web

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"pmjira/internal/store"
)

// maxUploadBytes caps a single attachment upload (50 MiB).
const maxUploadBytes = 50 << 20

// imageExts lists file extensions that browsers render inline as images.
var imageExts = map[string]bool{
	"png": true, "jpg": true, "jpeg": true, "gif": true,
	"webp": true, "svg": true, "bmp": true, "ico": true,
}

func lowerExt(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i < 0 {
		return ""
	}
	return strings.ToLower(name[i+1:])
}

// isImage reports whether a filename looks like a previewable image.
func isImage(name string) bool { return imageExts[lowerExt(name)] }

// isPDF reports whether a filename looks like a PDF.
func isPDF(name string) bool { return lowerExt(name) == "pdf" }

// previewable reports whether the file can be rendered inline (image or PDF).
func previewable(name string) bool { return isImage(name) || isPDF(name) }

// inlineContentType maps a filename to a MIME type for inline rendering.
func inlineContentType(name string) string {
	switch e := lowerExt(name); e {
	case "pdf":
		return "application/pdf"
	case "svg":
		return "image/svg+xml"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "bmp":
		return "image/bmp"
	case "ico":
		return "image/x-icon"
	case "txt", "md", "log":
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}

// handleAttachmentUpload handles POST /issues/{id}/attachments (multipart).
// Saves the file to disk, records it, and returns the refreshed attachment list.
func (s *Server) handleAttachmentUpload(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	if id == 0 {
		notFound(w)
		return
	}
	// Guard the body size before reading.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		badRequest(w, "文件过大或上传出错（上限 50MB）")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		badRequest(w, "请选择要上传的文件")
		return
	}
	defer file.Close()

	filename := strings.TrimSpace(header.Filename)
	if filename == "" {
		filename = "unnamed"
	}
	stored := store.NewStoredName(filename)
	dst := s.store.AttachmentPath(stored)
	out, err := os.Create(dst)
	if err != nil {
		serverError(w, err)
		return
	}
	n, err := io.Copy(out, file)
	out.Close()
	if err != nil {
		_ = os.Remove(dst)
		serverError(w, err)
		return
	}
	ct := header.Header.Get("Content-Type")
	if _, err := s.store.CreateAttachment(id, filename, stored, n, ct); err != nil {
		_ = os.Remove(dst)
		serverError(w, err)
		return
	}
	atts, _ := s.store.ListAttachments(id)
	s.r.RenderFragment(w, "attachments", map[string]any{"Attachments": atts, "IssueID": id, "IsAdmin": true})
}

// handleAttachmentDownload handles GET /attachments/{id}. The Content-Disposition
// prompts a download with the original filename.
func (s *Server) handleAttachmentDownload(w http.ResponseWriter, r *http.Request) {
	a, err := s.store.GetAttachment(idParam(r))
	if err != nil {
		notFound(w)
		return
	}
	s.serveAttachment(w, a, true)
}

// handleAttachmentView handles GET /attachments/{id}/view. It serves the file
// inline so the browser renders images/PDFs instead of downloading them.
func (s *Server) handleAttachmentView(w http.ResponseWriter, r *http.Request) {
	a, err := s.store.GetAttachment(idParam(r))
	if err != nil {
		notFound(w)
		return
	}
	s.serveAttachment(w, a, false)
}

// serveAttachment streams an attachment's file. When download is true the
// Content-Disposition forces a download; otherwise it is inline for preview.
func (s *Server) serveAttachment(w http.ResponseWriter, a store.Attachment, download bool) {
	f, err := os.Open(s.store.AttachmentPath(a.Stored))
	if err != nil {
		notFound(w)
		return
	}
	defer f.Close()

	// Prefer an extension-derived type so the browser renders inline correctly.
	w.Header().Set("Content-Type", inlineContentType(a.Filename))
	w.Header().Set("Content-Length", itoa(a.Size))
	if download {
		w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeFilename(a.Filename)+`"`)
	} else {
		w.Header().Set("Content-Disposition", `inline; filename="`+sanitizeFilename(a.Filename)+`"`)
	}
	_, _ = io.Copy(w, f)
}

// handleAttachmentDelete handles POST /attachments/{id}/delete.
func (s *Server) handleAttachmentDelete(w http.ResponseWriter, r *http.Request) {
	a, err := s.store.GetAttachment(idParam(r))
	if err != nil {
		notFound(w)
		return
	}
	stored, err := s.store.DeleteAttachment(a.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	if stored != "" {
		_ = os.Remove(s.store.AttachmentPath(stored))
	}
	redirect(w, r, "/issues/"+itoa(a.IssueID)+"?msg="+q("已删除附件"))
}

// sanitizeFilename strips path separators and control chars from a download name.
func sanitizeFilename(s string) string {
	s = filepath.Base(s)
	s = strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r < 0x20 {
			return '_'
		}
		return r
	}, s)
	return s
}
