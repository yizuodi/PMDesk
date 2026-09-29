package web

import (
	"encoding/csv"
	"net/http"
	"time"

	"pmjira/internal/store"
)

// exportFilename builds a timestamped, safe download filename.
func exportFilename(prefix string) string {
	return prefix + "-" + time.Now().Format("20060102-150405") + ".csv"
}

// writeIssuesCSV writes issues as CSV (with a UTF-8 BOM so Excel reads Chinese).
func writeIssuesCSV(w http.ResponseWriter, name string, issues []store.Issue) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	// UTF-8 BOM for Excel.
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	cw := csv.NewWriter(w)
	header := []string{"Key", "标题", "类型", "状态", "优先级", "截止日期", "创建时间", "更新时间"}
	_ = cw.Write(header)
	for _, i := range issues {
		_ = cw.Write([]string{
			i.Key, i.Title, typeLabel(i.Type), statusLabel(i.Status), priorityLabel(i.Priority),
			strPtrVal(i.DueDate), i.CreatedAt, i.UpdatedAt,
		})
	}
	cw.Flush()
}

// handleExportIssues exports the filtered issue list as CSV.
func (s *Server) handleExportIssues(w http.ResponseWriter, r *http.Request) {
	f := issueFilterFromQuery(r)
	issues, err := s.store.ListIssues(f)
	if err != nil {
		serverError(w, err)
		return
	}
	writeIssuesCSV(w, exportFilename("issues"), issues)
}
