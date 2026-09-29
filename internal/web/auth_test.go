package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigProviderCreatesDefaultsAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jira_config.json")
	cfg := NewConfigProvider(path)
	if cfg.ProtectionEnabled() {
		t.Fatal("new config unexpectedly enables protection")
	}
	if cfg.SessionDays() != 30 {
		t.Fatalf("default session days = %d, want 30", cfg.SessionDays())
	}

	writeConfig := func(password string, days int) {
		t.Helper()
		b, err := json.Marshal(appConfigFile{Password: password, SessionDays: days})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
		_ = os.Chtimes(path, time.Now().Add(2*time.Second), time.Now().Add(2*time.Second))
	}
	writeConfig("secret", 14)
	if cfg.Password() != "secret" || cfg.SessionDays() != 14 || !cfg.ProtectionEnabled() {
		t.Fatalf("config did not reload: password=%q days=%d", cfg.Password(), cfg.SessionDays())
	}
	writeConfig("new-secret", 0)
	if cfg.Password() != "new-secret" || cfg.SessionDays() != 30 {
		t.Fatalf("config defaulting/reload failed: password=%q days=%d", cfg.Password(), cfg.SessionDays())
	}
}

func TestAuthTokensAndSafeReturns(t *testing.T) {
	token := makeToken("secret", 1)
	if !verifyToken(token, "secret") {
		t.Fatal("valid token rejected")
	}
	if verifyToken(token, "wrong") {
		t.Fatal("token accepted with wrong password")
	}
	if verifyToken(makeToken("secret", -1), "secret") {
		t.Fatal("expired token accepted")
	}
	for _, input := range []string{"https://example.com", "//example.com", ""} {
		if got := safeReturn(input); got != "/" {
			t.Fatalf("safeReturn(%q) = %q, want /", input, got)
		}
	}
	if got := safeReturn("/issues/1?x=1"); got != "/issues/1?x=1" {
		t.Fatalf("safeReturn preserved path = %q", got)
	}
}

func TestAuthMiddlewareReadOnlyAndWriteBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jira_config.json")
	b, _ := json.Marshal(appConfigFile{Password: "secret", SessionDays: 30})
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := NewConfigProvider(path)
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	h := s.authMiddleware(base)

	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/issues", http.StatusNoContent},
		{http.MethodGet, "/issues/new", http.StatusSeeOther},
		{http.MethodGet, "/people", http.StatusSeeOther},
		{http.MethodPost, "/issues", http.StatusSeeOther},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("%s %s status = %d, want %d", tc.method, tc.path, w.Code, tc.want)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/issues", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous HTMX write status = %d, want 401", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/issues", nil)
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: makeToken("secret", 1)})
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("authenticated write status = %d, want 204", w.Code)
	}
}
