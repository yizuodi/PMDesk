package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pmjira/internal/store"
)

func openWebTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "pmjira.db"))
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "jira_config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"password":"","session_days":30}`), 0o600); err != nil {
		st.Close()
		t.Fatal(err)
	}
	srv := New(st, NewConfigProvider(cfgPath))
	t.Cleanup(func() { _ = st.Close() })
	return srv, st
}

func TestRoutesRenderRepresentativePages(t *testing.T) {
	srv, st := openWebTestServer(t)
	issue, err := st.CreateIssue(store.IssueInput{Title: "路由事务", Type: "task", Status: "todo", Priority: "medium"})
	if err != nil {
		t.Fatal(err)
	}
	version, err := st.CreateVersion(store.VersionInput{Name: "R027C10B001", StartDate: "2027-01-01", Status: "planned"})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	for _, tc := range []struct {
		path   string
		marker string
	}{
		{"/", "仪表盘"},
		{"/issues", "个人事务"},
		{"/issues/" + itoa(issue.ID), "路由事务"},
		{"/board", "看板"},
		{"/versions", "版本计划"},
		{"/versions?view=flat", "平铺列表"},
		{"/versions?view=roadmap", "版本路线图"},
		{"/versions/" + itoa(version.ID), "R027C10B001"},
		{"/people", "人员速查"},
		{"/settings", "设置"},
		{"/login", "登录"},
		{"/static/app.css", "--primary"},
	} {
		resp, err := http.Get(ts.URL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d", tc.path, resp.StatusCode)
		}
		if !strings.Contains(string(body), tc.marker) {
			t.Fatalf("GET %s missing marker %q", tc.path, tc.marker)
		}
	}
}

func TestIssueFilterNoLongerAcceptsRemovedRelationships(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/issues?version=12&assignee=alice&epic=4&type=bug", nil)
	filter := issueFilterFromQuery(req)
	if filter.Type != "bug" || filter.Search != "" || filter.Status != "" || filter.Priority != "" {
		t.Fatalf("current filters = %#v", filter)
	}
}
