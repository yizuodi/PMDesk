package web

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const authCookieName = "pmjira_auth"

// appConfigFile is the JSON shape of jira_config.json.
type appConfigFile struct {
	Password    string `json:"password"`
	SessionDays int    `json:"session_days"`
}

// ConfigProvider reads jira_config.json and reloads it when the file changes,
// so updating the password takes effect without restarting.
type ConfigProvider struct {
	path  string
	mu    sync.Mutex
	cfg   appConfigFile
	mtime time.Time
}

// NewConfigProvider loads the config (writing a sample file if absent) and
// returns a provider. An empty password disables protection.
func NewConfigProvider(path string) *ConfigProvider {
	p := &ConfigProvider{path: path}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// First run: drop a sample config next to the exe for the user to edit.
		sample := appConfigFile{Password: "", SessionDays: 30}
		if b, err := json.MarshalIndent(sample, "", "  "); err == nil {
			if err := os.WriteFile(path, b, 0o600); err == nil {
				log.Printf("已生成示例配置 %s — 设置 password 后即启用登录保护", path)
			}
		}
	}
	p.reload()
	return p
}

func (p *ConfigProvider) reload() {
	b, err := os.ReadFile(p.path)
	if err != nil {
		return
	}
	var c appConfigFile
	if err := json.Unmarshal(b, &c); err != nil {
		log.Printf("配置文件解析失败 %s: %v", p.path, err)
		return
	}
	if c.SessionDays <= 0 {
		c.SessionDays = 30
	}
	p.cfg = c
}

func (p *ConfigProvider) reloadIfChanged() {
	fi, err := os.Stat(p.path)
	if err != nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if fi.ModTime().Equal(p.mtime) {
		return
	}
	p.mtime = fi.ModTime()
	p.reload()
}

// Password returns the current configured password (reloaded on file change).
func (p *ConfigProvider) Password() string {
	p.reloadIfChanged()
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cfg.Password
}

// SessionDays returns the configured login lifetime (default 30).
func (p *ConfigProvider) SessionDays() int {
	p.reloadIfChanged()
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cfg.SessionDays
}

// ProtectionEnabled reports whether a password is configured.
func (p *ConfigProvider) ProtectionEnabled() bool { return p.Password() != "" }

// ---- token ----

// makeToken returns "expiry.signature" where the signature is HMAC-SHA256 of
// the expiry using the password as the key. Changing the password invalidates
// all outstanding tokens.
func makeToken(password string, days int) string {
	exp := time.Now().AddDate(0, 0, days).Unix()
	msg := strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, []byte(password))
	mac.Write([]byte(msg))
	return msg + "." + hex.EncodeToString(mac.Sum(nil))
}

// verifyToken validates a token against the current password and expiry.
func verifyToken(token, password string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	mac := hmac.New(sha256.New, []byte(password))
	mac.Write([]byte(parts[0]))
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(parts[1]), []byte(want))
}

type ctxKey int

const adminKey ctxKey = 0

func withAdmin(r *http.Request, admin bool) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), adminKey, admin))
}

// adminFromContext reads the admin flag set by the auth middleware.
func adminFromContext(r *http.Request) bool {
	v, _ := r.Context().Value(adminKey).(bool)
	return v
}

// isAdmin reports whether the request is from an authenticated (write-capable)
// user. With no password configured, everyone is an admin.
func (s *Server) isAdmin(r *http.Request) bool {
	pw := s.cfg.Password()
	if pw == "" {
		return true
	}
	c, err := r.Cookie(authCookieName)
	if err != nil {
		return false
	}
	return verifyToken(c.Value, pw)
}

// ---- middleware ----

// isFormPath reports whether a GET path renders a write form that an anonymous
// user should not see (new/edit/settings).
func isFormPath(p string) bool {
	if p == "/settings" {
		return true
	}
	if strings.HasSuffix(p, "/new") || strings.HasSuffix(p, "/edit") {
		return true
	}
	return false
}

// isLoginOnlyPath reports GET paths that are entirely login-gated (the whole
// module is admin-only, not just its write actions).
func isLoginOnlyPath(p string) bool {
	return strings.HasPrefix(p, "/people")
}

// isPublic reports paths that bypass the admin requirement entirely.
func isPublic(p string) bool {
	if p == "/login" || p == "/logout" {
		return true
	}
	return strings.HasPrefix(p, "/static/")
}

// authMiddleware gates write actions and form pages behind authentication,
// redirecting anonymous users to the login page (or 401 for HTMX requests).
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		admin := s.isAdmin(r)
		r = withAdmin(r, admin)

		needsAdmin := r.Method != http.MethodGet || isFormPath(r.URL.Path) || isLoginOnlyPath(r.URL.Path)
		if isPublic(r.URL.Path) {
			needsAdmin = false
		}
		if needsAdmin && !admin {
			if r.Header.Get("HX-Request") == "true" {
				http.Error(w, "需要登录", http.StatusUnauthorized)
				return
			}
			target := "/login?return=" + url.QueryEscape(safeReturn(r.URL.RequestURI()))
			http.Redirect(w, r, target, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// safeReturn guards against open redirects: only same-origin paths are allowed.
func safeReturn(s string) string {
	if s == "" || !strings.HasPrefix(s, "/") || strings.HasPrefix(s, "//") {
		return "/"
	}
	return s
}

// ---- handlers ----

// handleLogin renders the login form (GET) or validates the password (POST).
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		ret := safeReturn(r.URL.Query().Get("return"))
		m := s.base(r, "login", "登录")
		m["Return"] = ret
		m["LoginError"] = r.URL.Query().Get("err")
		m["ProtectionOff"] = !s.cfg.ProtectionEnabled()
		s.r.RenderPage(w, "login", m)
		return
	}

	ret := safeReturn(r.PostFormValue("return"))
	pw := s.cfg.Password()
	entered := r.PostFormValue("password")

	// No protection configured: any visit is admin; just proceed.
	if pw == "" {
		http.Redirect(w, r, ret, http.StatusSeeOther)
		return
	}
	if subtle.ConstantTimeCompare([]byte(entered), []byte(pw)) == 1 {
		days := s.cfg.SessionDays()
		http.SetCookie(w, &http.Cookie{
			Name:     authCookieName,
			Value:    makeToken(pw, days),
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   days * 24 * 3600,
		})
		http.Redirect(w, r, ret, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login?err="+q("密码错误")+"&return="+url.QueryEscape(ret), http.StatusSeeOther)
}

// handleLogout clears the auth cookie and returns home.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: authCookieName, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// DefaultConfigPath returns the config path next to the executable.
func DefaultConfigPath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "jira_config.json")
	}
	return "jira_config.json"
}
