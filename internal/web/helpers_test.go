package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIssueFormIgnoresRemovedRelationshipFields(t *testing.T) {
	req := httptest.NewRequest("POST", "/issues", strings.NewReader("title=事务&type=task&status=todo&priority=medium&assignee=alice&version_id=4&epic_id=5"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	in := issueInputFromForm(req)
	if in.Title != "事务" || in.Type != "task" || in.Status != "todo" || in.Priority != "medium" {
		t.Fatalf("current issue input = %#v", in)
	}
	if strings.Contains(strings.Join([]string{in.Title, in.Description}, " "), "alice") {
		t.Fatal("removed assignee value was persisted in input")
	}
}

func TestHistoricalActivityLabelsRemainReadable(t *testing.T) {
	for _, tc := range []struct {
		field string
		want  string
	}{
		{"assignee", "负责人"},
		{"version", "版本"},
		{"epic", "史诗"},
		{"description", "描述"},
	} {
		got := activityText(tc.field, "旧", "新")
		if !strings.Contains(got, tc.want) {
			t.Fatalf("activityText(%q) = %q, missing %q", tc.field, got, tc.want)
		}
	}
}
