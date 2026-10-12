package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigPINAndToken(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "orbit-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &Config{
		Port:      "8080",
		AuthToken: "initial-token-1234",
		DataDir:   tempDir,
	}

	// 1. Test Save
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}
	cfgFile := filepath.Join(tempDir, "server.config.json")
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		t.Fatalf("expected config file to exist at %s", cfgFile)
	}

	// 2. Test RotateToken
	oldToken := cfg.AuthToken
	newToken := cfg.RotateToken("")
	if newToken == oldToken || len(newToken) != 32 {
		t.Errorf("expected new random 32-char hex token, got %s", newToken)
	}

	custom := cfg.RotateToken("custom-token-xyz")
	if custom != "custom-token-xyz" || cfg.AuthToken != "custom-token-xyz" {
		t.Errorf("expected custom token to be set, got %s", custom)
	}

	// 3. Test PIN
	if !cfg.VerifyPIN("anything") {
		t.Errorf("expected VerifyPIN to succeed when no PIN is set")
	}

	if err := cfg.SetPIN("1234"); err != nil {
		t.Fatalf("failed to set PIN: %v", err)
	}
	if !cfg.VerifyPIN("1234") {
		t.Errorf("expected VerifyPIN to match '1234'")
	}
	if cfg.VerifyPIN("wrong") {
		t.Errorf("expected VerifyPIN to fail for 'wrong'")
	}

	// 4. Test RemovePIN
	if err := cfg.RemovePIN(); err != nil {
		t.Fatalf("failed to remove PIN: %v", err)
	}
	if cfg.PIN != "" {
		t.Errorf("expected PIN to be empty")
	}
}

func TestGetDataDirEnvVar(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "orbit-datadir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("ORBIT_DATA_DIR", tempDir)
	defer os.Unsetenv("ORBIT_DATA_DIR")

	got := GetDataDir()
	if filepath.Clean(got) != filepath.Clean(tempDir) {
		t.Errorf("expected GetDataDir to return ORBIT_DATA_DIR %s, got %s", tempDir, got)
	}
}

func TestCopyDir(t *testing.T) {
	srcDir, err := os.MkdirTemp("", "orbit-copydir-src-*")
	if err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	dstDir, err := os.MkdirTemp("", "orbit-copydir-dst-*")
	if err != nil {
		t.Fatalf("failed to create dst dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	subDir := filepath.Join(srcDir, "calendar_notes")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create sub dir: %v", err)
	}

	testFile := filepath.Join(subDir, "handoffs_registry.json")
	if err := os.WriteFile(testFile, []byte(`{"test":true}`), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	targetFile := filepath.Join(dstDir, "calendar_notes", "handoffs_registry.json")
	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("expected copied file at %s: %v", targetFile, err)
	}
	if string(data) != `{"test":true}` {
		t.Errorf("expected content %s, got %s", `{"test":true}`, string(data))
	}
}


