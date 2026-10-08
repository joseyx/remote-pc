package web

import (
	"net/http"
	"net/http/httptest"
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
