package store

import (
	"testing"
	"time"
)

func TestIssueLifecycleFiltersOrderingAndActivity(t *testing.T) {
	s, _ := openTestStore(t)
	past := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")
	soon := time.Now().AddDate(0, 0, 2).Format("2006-01-02")
	far := time.Now().AddDate(0, 0, 10).Format("2006-01-02")

	first, err := s.CreateIssue(IssueInput{Title: "逾期事务", Type: "task", Status: "in_progress", Priority: "high", DueDate: past})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateIssue(IssueInput{Title: "今日事务", Type: "bug", Status: "todo", Priority: "medium", DueDate: today})
	if err != nil {
		t.Fatal(err)
	}
	third, err := s.CreateIssue(IssueInput{Title: "即将到期", Type: "story", Status: "todo", Priority: "low", DueDate: soon})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateIssue(IssueInput{Title: "无截止日期", Type: "epic", Status: "done", Priority: "lowest"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateIssue(IssueInput{Title: "未来事务", Type: "task", Status: "todo", Priority: "medium", DueDate: far}); err != nil {
		t.Fatal(err)
	}

	issues, err := s.ListIssues(IssueFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 5 || issues[0].ID != first.ID || issues[1].ID != second.ID || issues[2].ID != third.ID {
		t.Fatalf("issue order = %#v", issues)
	}
	if issues[0].CommentCount != 0 {
		t.Fatalf("initial comment count = %d", issues[0].CommentCount)
	}
	if _, err := s.AddComment(first.ID, "需要跟进"); err != nil {
		t.Fatal(err)
	}
	issues, err = s.ListIssues(IssueFilter{Search: "逾期"})
	if err != nil || len(issues) != 1 || issues[0].CommentCount != 1 {
		t.Fatalf("search/comment result = %#v, err=%v", issues, err)
	}
	if got, err := s.ListIssues(IssueFilter{Type: "bug"}); err != nil || len(got) != 1 || got[0].ID != second.ID {
		t.Fatalf("type filter = %#v, err=%v", got, err)
	}
	if got, err := s.ListIssues(IssueFilter{Overdue: true}); err != nil || len(got) != 1 || got[0].ID != first.ID {
		t.Fatalf("overdue filter = %#v, err=%v", got, err)
	}
	if got, err := s.ListIssues(IssueFilter{DueWithin: 3}); err != nil || len(got) != 2 {
		t.Fatalf("due-soon filter = %#v, err=%v", got, err)
	}

	before, err := s.GetIssue(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := s.UpdateIssue(first.ID, IssueInput{
		Title: "逾期事务已更新", Type: before.Type, Status: "done", Priority: "highest",
		Description: "新的说明", DueDate: far,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "逾期事务已更新" || updated.Status != "done" || updated.Description != "新的说明" {
		t.Fatalf("updated issue = %#v", updated)
	}
	activities, err := s.ListActivities(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 5 {
		t.Fatalf("activity count = %d, want 5", len(activities))
	}
	if _, err := s.UpdateIssue(first.ID, IssueInput{
		Title: updated.Title, Type: updated.Type, Status: updated.Status, Priority: updated.Priority,
		Description: updated.Description, DueDate: far,
	}); err != nil {
		t.Fatal(err)
	}
	activitiesAgain, err := s.ListActivities(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(activitiesAgain) != len(activities) {
		t.Fatalf("no-op update added activities: %d -> %d", len(activities), len(activitiesAgain))
	}

	if err := s.SetIssueStatus(first.ID, "in_review"); err != nil {
		t.Fatal(err)
	}
	board, err := s.BoardColumns()
	if err != nil {
		t.Fatal(err)
	}
	if len(board["in_review"]) != 1 || board["in_review"][0].ID != first.ID {
		t.Fatalf("board columns = %#v", board)
	}
}

func TestIssueKeyAllocationAndDeleteCascade(t *testing.T) {
	s, _ := openTestStore(t)
	p, err := s.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	p.Key = "ABC"
	if err := s.UpdateProject(p.Name, p.Key); err != nil {
		t.Fatal(err)
	}
	issue, err := s.CreateIssue(IssueInput{Title: "事务", Type: "task", Status: "todo", Priority: "medium"})
	if err != nil {
		t.Fatal(err)
	}
	if issue.Key != "ABC-1" || issue.Seq != 1 {
		t.Fatalf("allocated issue = %#v", issue)
	}
	if _, err := s.AddComment(issue.ID, "评论"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`INSERT INTO activities(issue_id, field, old_value, new_value) VALUES(?,?,?,?)`, issue.ID, "status", "todo", "done"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteIssue(issue.ID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"comments", "activities", "attachments"} {
		var count int
		if err := s.DB.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE issue_id=?`, issue.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s count = %d after delete", table, count)
		}
	}
	var seq int
	if err := s.DB.QueryRow(`SELECT issue_seq FROM projects WHERE id=1`).Scan(&seq); err != nil {
		t.Fatal(err)
	}
	if seq != 1 {
		t.Fatalf("project sequence = %d, want 1", seq)
	}
}
