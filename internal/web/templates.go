package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"pmjira/internal/store"
)

//go:embed templates templates/* templates/*/* static/*.js static/*.css
var contentFS embed.FS

// pageFiles maps the logical page name (passed to RenderPage) to its template
// path under templates/. Each page file must {{define "page"}}.
var pageFiles = map[string]string{
	"dashboard":        "templates/dashboard.html",
	"login":            "templates/login.html",
	"nav_iframe":       "templates/nav_iframe.html",
	"settings":         "templates/settings.html",
	"issues/list":      "templates/issues/list.html",
	"issues/board":     "templates/issues/board.html",
	"issues/detail":    "templates/issues/detail.html",
	"issues/form":      "templates/issues/form.html",
	"people/list":      "templates/people/list.html",
	"people/form":      "templates/people/form.html",
	"versions/detail":  "templates/versions/detail.html",
	"versions/form":    "templates/versions/form.html",
	"versions/group":   "templates/versions/group.html",
	"versions/list":    "templates/versions/list.html",
	"versions/roadmap": "templates/versions/roadmap.html",
}

// Renderer parses templates once and renders layout-wrapped pages or bare
// fragments (for HTMX partial responses).
type Renderer struct {
	base     *template.Template // layout + partials; also renders fragments
	pageSets map[string]*template.Template
	markdown *markdownRenderer
}

// NewRenderer parses all templates. It panics on parse errors because a bad
// template is a programmer error, not a runtime one.
func NewRenderer() *Renderer {
	markdown := newMarkdownRenderer()
	fm := template.FuncMap{
		"statusLabel":        statusLabel,
		"statusClass":        statusClass,
		"typeLabel":          typeLabel,
		"typeClass":          typeClass,
		"priorityLabel":      priorityLabel,
		"priorityClass":      priorityClass,
		"versionStatusLabel": versionStatusLabel,
		"versionStatusClass": versionStatusClass,
		"dateFmt":            dateFmt,
		"relTime":            relTime,
		"daysFromToday":      daysFromToday,
		"dueLabel":           dueLabel,
		"dueClass":           dueClass,
		"lower":              strings.ToLower,
		"upper":              strings.ToUpper,
		"dict":               dict,
		"statorder":          statusOrder,
		"activityText":       activityText,
		"inc":                func(n int) int { return n + 1 },
		"humanSize":          humanSize,
		"isImage":            isImage,
		"isPDF":              isPDF,
		"previewable":        previewable,
		"vtypeLabel":         vtypeLabel,
		"vtypeClass":         vtypeClass,
		"versionParts":       versionParts,
		"markdownHTML":       markdown.HTML,
		"markdownSummary":    markdown.Summary,
	}

	base := template.Must(template.New("base").
		Funcs(fm).
		ParseFS(contentFS, "templates/layout.html", "templates/partials/*.html"))

	r := &Renderer{
		base:     base,
		pageSets: make(map[string]*template.Template),
		markdown: markdown,
	}
	for name, file := range pageFiles {
		clone, err := base.Clone()
		if err != nil {
			panic(err)
		}
		t, err := clone.ParseFS(contentFS, file)
		if err != nil {
			panic(fmt.Errorf("parse %s: %w", file, err))
		}
		r.pageSets[name] = t
	}
	return r
}

// RenderPage renders a layout-wrapped page with the given data map. If the map
// lacks the common keys (Project/Nav/Title), the caller is responsible for them.
func (r *Renderer) RenderPage(w http.ResponseWriter, page string, data map[string]any) {
	t, ok := r.pageSets[page]
	if !ok {
		http.Error(w, "unknown page "+page, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout.html", data); err != nil {
		fmt.Println("render page error:", err)
	}
}

// RenderFragment renders a single named template (defined via {{define}}) with
// no layout, for HTMX partial responses.
func (r *Renderer) RenderFragment(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := r.base.ExecuteTemplate(w, name, data); err != nil {
		fmt.Println("render fragment error:", err)
	}
}

// StaticFS exposes the embedded static sub-filesystem for serving via http.FileServer.
func StaticFS() fs.FS {
	sub, err := fs.Sub(contentFS, "static")
	if err != nil {
		panic(err)
	}
	return sub
}

// ---- label / class helpers -------------------------------------------------

var (
	statusZh = map[string]string{"todo": "待办", "in_progress": "进行中", "in_review": "审核中", "done": "已完成"}
	typeZh   = map[string]string{"epic": "史诗", "story": "故事", "task": "任务", "bug": "缺陷"}
	prioZh   = map[string]string{"highest": "最高", "high": "高", "medium": "中", "low": "低", "lowest": "最低"}
	verZh    = map[string]string{"planned": "已计划", "active": "进行中", "tested": "已转测", "released": "已发布"}

	statusCls = map[string]string{"todo": "s-todo", "in_progress": "s-progress", "in_review": "s-review", "done": "s-done"}
	typeCls   = map[string]string{"epic": "t-epic", "story": "t-story", "task": "t-task", "bug": "t-bug"}
	prioCls   = map[string]string{"highest": "p-highest", "high": "p-high", "medium": "p-medium", "low": "p-low", "lowest": "p-lowest"}
	verCls    = map[string]string{"planned": "v-planned", "active": "v-active", "tested": "v-tested", "released": "v-released"}
)

func statusLabel(s string) string        { return statusZh[s] }
func statusClass(s string) string        { return statusCls[s] }
func typeLabel(s string) string          { return typeZh[s] }
func typeClass(s string) string          { return typeCls[s] }
func priorityLabel(s string) string      { return prioZh[s] }
func priorityClass(s string) string      { return prioCls[s] }
func versionStatusLabel(s string) string { return verZh[s] }
func versionStatusClass(s string) string { return verCls[s] }

// statusOrder returns the board column order for a status (0..3).
func statusOrder(s string) int {
	switch s {
	case "todo":
		return 0
	case "in_progress":
		return 1
	case "in_review":
		return 2
	case "done":
		return 3
	}
	return 4
}

func dateFmt(v any) string {
	s := asString(v)
	if s == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return t.Format("2006-01-02")
}

// asString normalises string / *string / nil template values to a plain string.
func asString(v any) string {
	switch s := v.(type) {
	case nil:
		return ""
	case string:
		return s
	case *string:
		if s == nil {
			return ""
		}
		return *s
	}
	return fmt.Sprintf("%v", v)
}

// relTime renders a human-friendly relative time for an RFC3339-ish timestamp.
func relTime(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// tolerate the "YYYY-MM-DDTHH:MM:SSZ" produced by strftime
		t, err = time.Parse("2006-01-02T15:04:05Z", s)
		if err != nil {
			return s
		}
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d 天前", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

// daysFromToday returns days between today and an ISO date (negative = overdue).
func daysFromToday(v any) int {
	iso := asString(v)
	if iso == "" {
		return 1 << 30 // far future, effectively "not relevant"
	}
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return 1 << 30
	}
	today := time.Now().Format("2006-01-02")
	t0, _ := time.Parse("2006-01-02", today)
	return int(t.Sub(t0).Hours() / 24)
}

// dueLabel renders a short urgency label for a due date, "" when not urgent.
func dueLabel(v any) string {
	d := daysFromToday(v)
	switch {
	case d == 1<<30:
		return ""
	case d < 0:
		return fmt.Sprintf("超期 %d 天", -d)
	case d == 0:
		return "今日到期"
	case d <= 3:
		return fmt.Sprintf("还剩 %d 天", d)
	default:
		return ""
	}
}

// dueClass returns the CSS class for a due-date urgency flag.
func dueClass(v any) string {
	d := daysFromToday(v)
	switch {
	case d < 0, d == 0:
		return "overdue-flag"
	case d <= 3:
		return "row-due soon"
	default:
		return ""
	}
}

// dict creates a map from key/value pairs, useful in templates.
func dict(values ...any) map[string]any {
	m := make(map[string]any, len(values)/2)
	for i := 0; i+1 < len(values); i += 2 {
		k, _ := values[i].(string)
		m[k] = values[i+1]
	}
	return m
}

var fieldZh = map[string]string{
	"status": "状态", "type": "类型", "priority": "优先级", "assignee": "负责人",
	"version": "版本", "epic": "史诗", "title": "标题", "description": "描述", "due_date": "截止日期",
}

// humanSize renders a byte count as a friendly string (B / KiB / MiB …).
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	units := "KMGTPE"
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), units[exp])
}

// vtypeLabel/vtypeClass render the structured-version type as a Chinese label
// and a CSS badge class.
func vtypeLabel(v string) string {
	switch v {
	case store.VTypeC:
		return "C版本"
	case store.VTypeB:
		return "内部B"
	case store.VTypeSPC:
		return "预发布 SPC"
	case store.VTypeCP:
		return "预发布 CP"
	case store.VTypeKey:
		return "关键节点"
	}
	return "其他"
}

func vtypeClass(v string) string {
	switch v {
	case store.VTypeC:
		return "vt-c"
	case store.VTypeB:
		return "vt-b"
	case store.VTypeSPC:
		return "vt-spc"
	case store.VTypeCP:
		return "vt-cp"
	case store.VTypeKey:
		return "vt-key"
	}
	return "vt-other"
}

// versionParts renders the parsed numeric components of a version, e.g.
// "R027 · C10 · B003" or "SPC200 · B020". Empty for non-conforming versions.
func versionParts(v store.Version) string {
	deref := func(p *int) int {
		if p == nil {
			return 0
		}
		return *p
	}
	switch v.VType {
	case store.VTypeC:
		if v.RNum == nil || v.CNum == nil {
			return ""
		}
		return fmt.Sprintf("R%03d · C%02d", deref(v.RNum), deref(v.CNum))
	case store.VTypeKey:
		if v.RNum == nil || v.CNum == nil {
			return ""
		}
		return fmt.Sprintf("R%03d · C%02d", deref(v.RNum), deref(v.CNum))
	case store.VTypeB:
		if v.RNum == nil || v.CNum == nil {
			return ""
		}
		return fmt.Sprintf("R%03d · C%02d · B%03d", deref(v.RNum), deref(v.CNum), deref(v.BNum))
	case store.VTypeSPC:
		return fmt.Sprintf("R%03d · C%02d · SPC%03d · B%03d", deref(v.RNum), deref(v.CNum), deref(v.ExtraNum), deref(v.BNum))
	case store.VTypeCP:
		return fmt.Sprintf("R%03d · C%02d · CP%04d · B%03d", deref(v.RNum), deref(v.CNum), deref(v.ExtraNum), deref(v.BNum))
	}
	return ""
}

// activityText renders a single change entry as a readable Chinese phrase.
func activityText(field, oldV, newV string) string {
	label := func(v string) string {
		if v == "" || v == "0" {
			return "无"
		}
		switch field {
		case "status":
			return statusLabel(v)
		case "type":
			return typeLabel(v)
		case "priority":
			return priorityLabel(v)
		}
		return v
	}
	fieldLabel := fieldZh[field]
	if fieldLabel == "" {
		fieldLabel = field
	}
	return fmt.Sprintf("%s：%s → %s", fieldLabel, label(oldV), label(newV))
}
