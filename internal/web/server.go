// Package web serves the PM Jira web UI.
package web

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"pmjira/internal/store"
)

// AppVersion is shown in the footer.
const AppVersion = "1.0.0"

// Server wires the store and renderer to HTTP routes.
type Server struct {
	store      *store.Store
	r          *Renderer
	mux        *http.ServeMux
	cfg        *ConfigProvider
	shutdownCh chan struct{} // closed when /shutdown is requested
}

// New creates a Server with all routes registered.
func New(s *store.Store, cfg *ConfigProvider) *Server {
	srv := &Server{
		store:      s,
		r:          NewRenderer(),
		mux:        http.NewServeMux(),
		cfg:        cfg,
		shutdownCh: make(chan struct{}),
	}
	srv.routes()
	return srv
}

// ShutdownCh returns a channel that is closed when the shutdown endpoint is hit.
// The caller (main) treats it as a signal to stop the HTTP server.
func (s *Server) ShutdownCh() <-chan struct{} { return s.shutdownCh }

// Handler returns the root http.Handler (with middleware applied).
func (s *Server) Handler() http.Handler {
	return s.recoverer(s.logger(s.authMiddleware(s.mux)))
}

// routes registers all URL patterns. Uses Go 1.22+ method-aware patterns.
func (s *Server) routes() {
	mux := s.mux

	// Static assets (embedded).
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(StaticFS()))))

	// Dashboard
	mux.HandleFunc("GET /{$}", s.handleDashboard)

	// Issues
	mux.HandleFunc("GET /issues", s.handleIssuesList)
	mux.HandleFunc("GET /issues/new", s.handleIssueForm)
	mux.HandleFunc("POST /issues", s.handleIssueCreate)
	mux.HandleFunc("GET /issues/{id}", s.handleIssueDetail)
	mux.HandleFunc("GET /issues/{id}/edit", s.handleIssueForm)
	mux.HandleFunc("POST /issues/{id}", s.handleIssueUpdate)
	mux.HandleFunc("POST /issues/{id}/delete", s.handleIssueDelete)
	mux.HandleFunc("POST /issues/{id}/status", s.handleIssueStatus) // HTMX board move
	mux.HandleFunc("POST /issues/{id}/comments", s.handleCommentAdd)
	mux.HandleFunc("POST /comments/{id}/delete", s.handleCommentDelete)
	mux.HandleFunc("POST /issues/{id}/attachments", s.handleAttachmentUpload)
	mux.HandleFunc("GET /attachments/{id}", s.handleAttachmentDownload)
	mux.HandleFunc("GET /attachments/{id}/view", s.handleAttachmentView)
	mux.HandleFunc("POST /attachments/{id}/delete", s.handleAttachmentDelete)

	// Board
	mux.HandleFunc("GET /board", s.handleBoard)

	// Reports / export
	mux.HandleFunc("GET /issues/export.csv", s.handleExportIssues)

	// Shared Markdown editor preview (write access only via auth middleware).
	mux.HandleFunc("POST /markdown/preview", s.handleMarkdownPreview)

	// Versions
	mux.HandleFunc("GET /versions", s.handleVersionsList)
	mux.HandleFunc("GET /versions/new", s.handleVersionForm)
	mux.HandleFunc("POST /versions", s.handleVersionCreate)
	mux.HandleFunc("GET /versions/{id}", s.handleVersionDetail)
	mux.HandleFunc("GET /versions/{id}/edit", s.handleVersionForm)
	mux.HandleFunc("POST /versions/{id}", s.handleVersionUpdate)
	mux.HandleFunc("POST /versions/{id}/delete", s.handleVersionDelete)

	// Settings
	mux.HandleFunc("GET /settings", s.handleSettings)
	mux.HandleFunc("POST /settings", s.handleSettingsSave)

	// Custom quick-nav links
	mux.HandleFunc("GET /nav/{id}", s.handleNavEmbed)
	mux.HandleFunc("POST /settings/nav", s.handleNavCreate)
	mux.HandleFunc("POST /settings/nav/{id}/update", s.handleNavUpdate)
	mux.HandleFunc("POST /settings/nav/{id}/delete", s.handleNavDelete)
	mux.HandleFunc("POST /settings/nav/{id}/{dir}", s.handleNavMove)

	// Service control (used by the no-console exe to stop cleanly)
	mux.HandleFunc("POST /shutdown", s.handleShutdown)

	// Authentication
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("GET /logout", s.handleLogout)

	// 人员速查 (login-only module)
	mux.HandleFunc("GET /people", s.handlePeopleList)
	mux.HandleFunc("GET /people/new", s.handlePersonForm)
	mux.HandleFunc("POST /people", s.handlePersonCreate)
	mux.HandleFunc("GET /people/{id}/edit", s.handlePersonForm)
	mux.HandleFunc("POST /people/{id}", s.handlePersonUpdate)
	mux.HandleFunc("POST /people/{id}/delete", s.handlePersonDelete)
	mux.HandleFunc("POST /people/contacts", s.handleContactCreate)
	mux.HandleFunc("POST /people/contacts/{id}/update", s.handleContactUpdate)
	mux.HandleFunc("POST /people/contacts/{id}/delete", s.handleContactDelete)
	mux.HandleFunc("POST /people/contacts/{id}/{dir}", s.handleContactMove)
}

// handleShutdown signals the server to stop. It is how users exit the
// no-console (windowsgui) build, since there is no terminal for Ctrl+C.
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<!DOCTYPE html><meta charset=utf-8><title>已停止</title>" +
		"<body style=\"font-family:sans-serif;text-align:center;padding:60px\">" +
		"<h2>PM Jira 已停止</h2><p>可以关闭这个页面了。</p></body>"))
	go func() {
		select {
		case <-s.shutdownCh:
		default:
			close(s.shutdownCh)
		}
	}()
}

// base returns the common view envelope: project settings, active nav, title,
// custom nav links (flat + grouped by category), and any flash message.
func (s *Server) base(r *http.Request, nav, title string) map[string]any {
	p, _ := s.store.GetProject()
	navLinks, _ := s.store.ListNavLinks()
	flash := r.URL.Query().Get("msg")
	return map[string]any{
		"Project":  p,
		"Nav":      nav,
		"Title":    title,
		"NavLinks": navLinks,
		"NavCats":  groupNavByCategory(navLinks),
		"Flash":    flash,
		"Version":  AppVersion,
		"Year":     time.Now().Year(),
		"IsAdmin":  adminFromContext(r),
	}
}

// navCat is a category group for the top-bar hover submenu.
type navCat struct {
	Cat   string
	Links []store.NavLink
}

// groupNavByCategory collects nav links with a non-empty category, preserving
// first-seen order of categories. Links with no category are rendered directly
// in the template (via .NavLinks), so they're excluded here.
func groupNavByCategory(links []store.NavLink) []navCat {
	var cats []navCat
	idx := map[string]int{}
	for _, l := range links {
		if l.Category == "" {
			continue
		}
		if pos, ok := idx[l.Category]; ok {
			cats[pos].Links = append(cats[pos].Links, l)
		} else {
			idx[l.Category] = len(cats)
			cats = append(cats, navCat{Cat: l.Category, Links: []store.NavLink{l}})
		}
	}
	return cats
}

// ---- middleware -----------------------------------------------------------

func (s *Server) logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.String(), time.Since(start).Round(time.Millisecond))
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC %s %s: %v", r.Method, r.URL, rec)
				http.Error(w, "内部错误", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ---- helpers --------------------------------------------------------------

// idParam parses the {id} path value as int64.
func idParam(r *http.Request) int64 {
	v, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return v
}

// atoiOr is a small convenience.
func atoiOr(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
