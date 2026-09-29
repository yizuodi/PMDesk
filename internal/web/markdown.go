package web

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	xhtml "golang.org/x/net/html"
)

const (
	markdownPreviewLimit = 1 << 20
	markdownSummaryLimit = 180
)

type markdownRenderer struct {
	markdown goldmark.Markdown
	policy   *bluemonday.Policy
}

func newMarkdownRenderer() *markdownRenderer {
	policy := bluemonday.NewPolicy()
	policy.AllowElements(
		"p", "br", "hr",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"strong", "em", "del",
		"ul", "ol", "li", "blockquote",
		"code", "pre",
		"table", "thead", "tbody", "tr", "th", "td",
		"a", "input",
	)
	policy.AllowAttrs("href", "title").OnElements("a")
	policy.AllowURLSchemes("http", "https", "mailto")
	policy.AllowRelativeURLs(true)
	policy.AllowAttrs("align").Matching(regexp.MustCompile(`^(left|center|right)$`)).OnElements("th", "td")
	policy.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	policy.AllowAttrs("checked", "disabled").OnElements("input")

	return &markdownRenderer{
		markdown: goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithRendererOptions(goldmarkhtml.WithHardWraps()),
		),
		policy: policy,
	}
}

// HTML is the only trusted-HTML boundary for persisted Markdown. Goldmark does
// not pass raw HTML through, and Bluemonday applies a second, explicit allowlist
// before the result is marked safe for html/template.
func (r *markdownRenderer) HTML(source string) template.HTML {
	var rendered bytes.Buffer
	if err := r.markdown.Convert([]byte(source), &rendered); err != nil {
		return template.HTML(template.HTMLEscapeString(source))
	}
	return template.HTML(r.policy.SanitizeBytes(rendered.Bytes()))
}

func (r *markdownRenderer) Summary(source string) string {
	safe := string(r.HTML(source))
	z := xhtml.NewTokenizer(strings.NewReader(safe))
	var text strings.Builder
	for {
		tt := z.Next()
		switch tt {
		case xhtml.ErrorToken:
			return truncateSummary(strings.Join(strings.Fields(text.String()), " "), markdownSummaryLimit)
		case xhtml.TextToken:
			text.Write(z.Text())
			text.WriteByte(' ')
		case xhtml.StartTagToken, xhtml.EndTagToken, xhtml.SelfClosingTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "br", "p", "li", "blockquote", "pre", "tr", "h1", "h2", "h3", "h4", "h5", "h6":
				text.WriteByte(' ')
			}
		}
	}
}

func truncateSummary(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	cut := strings.TrimRightFunc(string(runes[:limit]), unicode.IsSpace)
	return cut + "…"
}

func (s *Server) handleMarkdownPreview(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, markdownPreviewLimit)
	if err := r.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "内容不能超过 1 MiB", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "无法读取 Markdown 内容", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	source := r.PostFormValue("markdown")
	if strings.TrimSpace(source) == "" {
		_, _ = w.Write([]byte(`<div class="muted">暂无内容</div>`))
		return
	}
	_, _ = w.Write([]byte(s.r.markdown.HTML(source)))
}
