package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orbit/internal/config"
	"orbit/internal/session"
)

// setupClickUpServer starts a server whose ClickUp store already holds one pending mention, c1.
func setupClickUpServer(t *testing.T) http.Handler {
	t.Helper()
	// Like setupTestServer: the server's background savers may still write here when the test ends,
	// so the directory is removed best-effort instead of through t.TempDir.
	dir, err := os.MkdirTemp("", "orbit-clickup-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	at := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	seed := fmt.Sprintf(`{"version":1,"mentions":[{"commentId":"c1","taskId":"abc001","taskName":"Tarea A",`+
		`"author":"Ana","text":"revisa","date":%q,"detectedAt":%q,"reason":"mencion","answered":false}]}`, at, at)
	if err := os.WriteFile(filepath.Join(dir, clickupStoreFile), []byte(seed), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Port: "8080", AuthToken: "test-secret-token", DataDir: dir}
	srv := NewServer(cfg, session.NewManager(1024, filepath.Join(dir, "sessions_state.json")), "1.0.0")
	return srv.Routes()
}

func clickupCall(t *testing.T, h http.Handler, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-secret-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestClickUpInboxDoneEndpoint(t *testing.T) {
	h := setupClickUpServer(t)

	code, out := clickupCall(t, h, "GET", "/api/clickup/mentions", "")
	if code != http.StatusOK || out["pendingCount"] != float64(1) || out["dailyNote"] != false || len(out["mentions"].([]any)) != 1 {
		t.Fatalf("inbox = %d %v", code, out)
	}

	// An empty body marks it attended.
	if code, out = clickupCall(t, h, "POST", "/api/clickup/mentions/c1/done", ""); code != http.StatusOK || out["pendingCount"] != float64(0) {
		t.Fatalf("done = %d %v", code, out)
	}
	if _, out = clickupCall(t, h, "GET", "/api/clickup/mentions", ""); len(out["mentions"].([]any)) != 0 {
		t.Errorf("attended item still listed as pending: %v", out)
	}
	if _, out = clickupCall(t, h, "GET", "/api/clickup/mentions?include_answered=1", ""); len(out["mentions"].([]any)) != 1 {
		t.Errorf("attended item missing from the full list: %v", out)
	}
	if code, out = clickupCall(t, h, "POST", "/api/clickup/mentions/c1/done", `{"done": false}`); code != http.StatusOK || out["pendingCount"] != float64(1) {
		t.Fatalf("reopen = %d %v", code, out)
	}

	for _, c := range []struct {
		path, body string
		want       int
	}{
		{"/api/clickup/mentions/nope/done", "", http.StatusNotFound},
		{"/api/clickup/mentions/a.b/done", "", http.StatusBadRequest},
		{"/api/clickup/mentions/c1/done", `{"done": "yes"}`, http.StatusBadRequest},
	} {
		if code, _ := clickupCall(t, h, "POST", c.path, c.body); code != c.want {
			t.Errorf("POST %s %s = %d, want %d", c.path, c.body, code, c.want)
		}
	}

	req := httptest.NewRequest("POST", "/api/clickup/mentions/c1/done", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("without token = %d, want 401", w.Code)
	}
}

func TestClickUpPollValidatesLookback(t *testing.T) {
	h := setupClickUpServer(t)
	for _, body := range []string{`{"lookback_hours": 0}`, `{"lookback_hours": 1.5}`, `{"lookback_hours": 169}`, `{"lookback_hours": "72"}`, `not json`} {
		if code, _ := clickupCall(t, h, "POST", "/api/clickup/poll", body); code != http.StatusBadRequest {
			t.Errorf("poll %s = %d, want 400", body, code)
		}
	}
	// A valid body reaches the poller, which refuses because the plugin is off.
	for _, body := range []string{`{"lookback_hours": 72}`, ""} {
		if code, _ := clickupCall(t, h, "POST", "/api/clickup/poll", body); code != http.StatusConflict {
			t.Errorf("poll %q = %d, want 409 (plugin disabled)", body, code)
		}
	}
}
