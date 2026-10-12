package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"orbit/internal/config"
	"orbit/internal/session"
)

func setupTestServer(t *testing.T) (*Server, func()) {
	tempDir, err := os.MkdirTemp("", "orbit-api-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := &config.Config{
		Port:      "8080",
		AuthToken: "test-secret-token",
		DataDir:   tempDir,
	}

	mgr := session.NewManager(1024, filepath.Join(tempDir, "sessions_state.json"))
	srv := NewServer(cfg, mgr, "1.0.0")

	cleanup := func() {
		os.RemoveAll(tempDir)
	}
	return srv, cleanup
}

func TestSecurityEndpoints(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	routes := srv.Routes()

	// 1. Test GET /api/system/security (Unauthorized without token)
	req := httptest.NewRequest("GET", "/api/system/security", nil)
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", w.Code)
	}

	// 2. Test GET /api/system/security (Authorized)
	req = httptest.NewRequest("GET", "/api/system/security", nil)
	req.Header.Set("Authorization", "Bearer test-secret-token")
	w = httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	var secStatus struct {
		Token  string `json:"token"`
		HasPin bool   `json:"hasPin"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &secStatus)
	if secStatus.Token != "test-secret-token" || secStatus.HasPin {
		t.Errorf("unexpected security status: %+v", secStatus)
	}

	// 3. Test POST /api/system/pin/set
	pinPayload, _ := json.Marshal(map[string]string{
		"newPin": "4829",
	})
	req = httptest.NewRequest("POST", "/api/system/pin/set", bytes.NewReader(pinPayload))
	req.Header.Set("Authorization", "Bearer test-secret-token")
	w = httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK on setting PIN, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Test POST /api/system/pin/verify
	verifyPayload, _ := json.Marshal(map[string]string{
		"pin": "4829",
	})
	req = httptest.NewRequest("POST", "/api/system/pin/verify", bytes.NewReader(verifyPayload))
	w = httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK on verify PIN, got %d", w.Code)
	}
	var verResult struct {
		Valid bool `json:"valid"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &verResult)
	if !verResult.Valid {
		t.Errorf("expected PIN 4829 to be valid")
	}

	// 5. Test POST /api/system/token/rotate with wrong PIN -> should fail 403
	rotatePayload, _ := json.Marshal(map[string]string{
		"pin": "0000",
	})
	req = httptest.NewRequest("POST", "/api/system/token/rotate", bytes.NewReader(rotatePayload))
	req.Header.Set("Authorization", "Bearer test-secret-token")
	w = httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden with wrong PIN, got %d", w.Code)
	}

	// 6. Test POST /api/system/token/rotate with correct PIN -> succeeds
	rotatePayload, _ = json.Marshal(map[string]string{
		"pin": "4829",
	})
	req = httptest.NewRequest("POST", "/api/system/token/rotate", bytes.NewReader(rotatePayload))
	req.Header.Set("Authorization", "Bearer test-secret-token")
	w = httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK with correct PIN, got %d", w.Code)
	}

	var rotateResult struct {
		Success bool   `json:"success"`
		Token   string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &rotateResult)
	if !rotateResult.Success || rotateResult.Token == "test-secret-token" {
		t.Errorf("unexpected rotate result: %+v", rotateResult)
	}

	// 7. Verify old token is now rejected and new token is accepted
	req = httptest.NewRequest("GET", "/api/system/security", nil)
	req.Header.Set("Authorization", "Bearer test-secret-token")
	w = httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected old token to be rejected with 401, got %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/api/system/security", nil)
	req.Header.Set("Authorization", "Bearer "+rotateResult.Token)
	w = httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected new token to be accepted with 200, got %d", w.Code)
	}
}
