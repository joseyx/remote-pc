package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandler(t *testing.T) {
	h := Handler()

	paths := []string{
		"/",
		"/xterm.js",
		"/marked.min.js",
		"/css/base.css",
		"/css/layout.css",
		"/css/bubbles.css",
		"/css/terminal.css",
		"/css/views.css",
		"/css/calendar.css",
		"/css/modals.css",
		"/css/responsive.css",
		"/js/state.js",
		"/js/terminal.js",
		"/js/workspaces.js",
		"/js/actions.js",
		"/js/views.js",
		"/js/calendar.js",
		"/js/palette.js",
		"/js/system.js",
		"/js/app.js",
	}

	for _, p := range paths {
		req := httptest.NewRequest("GET", p, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 for %s, got %d", p, rec.Code)
		}
	}
}

func TestExtractStaticAssetsAndOverride(t *testing.T) {
	tempDir := t.TempDir()
	staticDir := filepath.Join(tempDir, "static")

	if err := ExtractStaticAssets(staticDir); err != nil {
		t.Fatalf("ExtractStaticAssets failed: %v", err)
	}

	indexPath := filepath.Join(staticDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("extracted index.html does not exist: %v", err)
	}

	// Modify extracted index.html to verify hot reload
	customContent := []byte("<!DOCTYPE html><html><body>HOT_RELOAD_TEST</body></html>")
	if err := os.WriteFile(indexPath, customContent, 0644); err != nil {
		t.Fatalf("failed to write custom index.html: %v", err)
	}

	// Handler with dataDir override
	h := Handler(tempDir)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != string(customContent) {
		t.Errorf("expected hot reloaded body %q, got %q", string(customContent), rec.Body.String())
	}
}
