package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestMarkdownHTML(t *testing.T) {
	r := newMarkdownRenderer()
	got := string(r.HTML("# 标题\n\n**粗体**\n\n- [x] 完成\n\n| A | B |\n|---|---|\n| 1 | 2 |"))

	for _, want := range []string{"<h1>标题</h1>", "<strong>粗体</strong>", "type=\"checkbox\"", "<table>"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered Markdown missing %q:\n%s", want, got)
		}
	}
}

func TestMarkdownHTMLPreservesPlainTextLineBreaks(t *testing.T) {
	got := string(newMarkdownRenderer().HTML("第一行\n第二行"))
	if !strings.Contains(got, "第一行<br") || !strings.Contains(got, "第二行") {
		t.Fatalf("plain-text line break was not preserved: %s", got)
	}
}

func TestMarkdownHTMLSanitizesUnsafeContent(t *testing.T) {
	r := newMarkdownRenderer()
	got := string(r.HTML(strings.Join([]string{
		`<script>alert(1)</script>`,
		`<iframe src="https://example.com"></iframe>`,
		`<img src=x onerror="alert(1)">`,
		`[危险链接](javascript:alert(1))`,
		`![远程图片](https://example.com/track.png)`,
		`[安全链接](https://example.com)`,
	}, "\n\n")))

	for _, unsafe := range []string{"<script", "<iframe", "<img", "onerror", "javascript:", "track.png"} {
		if strings.Contains(strings.ToLower(got), unsafe) {
			t.Errorf("unsafe content %q survived sanitization:\n%s", unsafe, got)
		}
	}
	if !strings.Contains(got, `href="https://example.com"`) || !strings.Contains(got, "安全链接") {
		t.Errorf("safe link was removed: %s", got)
	}
}

func TestMarkdownSummary(t *testing.T) {
	r := newMarkdownRenderer()
	got := r.Summary("# 标题\n\n这是 **粗体** 和 [链接文字](https://example.com)。\n\n`代码`")
	if got != "标题 这是 粗体 和 链接文字 。 代码" {
		t.Fatalf("unexpected summary: %q", got)
	}
}

func TestTruncateSummaryUsesRunes(t *testing.T) {
	got := truncateSummary("一二三四五", 3)
	if got != "一二三…" {
		t.Fatalf("unexpected Unicode truncation: %q", got)
	}
}

func TestMarkdownPreview(t *testing.T) {
	s := &Server{r: NewRenderer()}
	form := url.Values{"markdown": {"## 预览\n\n**内容**"}}
	req := httptest.NewRequest(http.MethodPost, "/markdown/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	s.handleMarkdownPreview(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q", got)
	}
	if body := w.Body.String(); !strings.Contains(body, "<h2>预览</h2>") || !strings.Contains(body, "<strong>内容</strong>") {
		t.Errorf("unexpected preview body: %s", body)
	}
}

func TestMarkdownPreviewRejectsOversizedContent(t *testing.T) {
	s := &Server{r: NewRenderer()}
	form := url.Values{"markdown": {strings.Repeat("a", markdownPreviewLimit+1)}}
	req := httptest.NewRequest(http.MethodPost, "/markdown/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	s.handleMarkdownPreview(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("preview status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestMarkdownEditorEscapesTextareaContent(t *testing.T) {
	r := NewRenderer()
	w := httptest.NewRecorder()
	data := map[string]any{
		"ID":          "description",
		"Name":        "description",
		"Label":       "描述",
		"Placeholder": "输入描述",
		"Value":       `</textarea><script>alert(1)</script>`,
	}

	if err := r.base.ExecuteTemplate(w, "markdown_editor", data); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if strings.Contains(body, "</textarea><script>") || !strings.Contains(body, "&lt;/textarea&gt;") {
		t.Fatalf("textarea content escaped incorrectly: %s", body)
	}
}
