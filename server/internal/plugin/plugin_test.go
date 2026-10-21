package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPluginManager(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	tempDir := t.TempDir()

	mgr := NewManager(tempDir)

	// 1. Initial plugins list
	plugins := mgr.ListPlugins(false)
	if len(plugins) == 0 {
		t.Fatalf("Expected default plugins in registry, found 0")
	}

	foundGroq := false
	for _, p := range plugins {
		if p.ID == "groq_whisper" {
			foundGroq = true
			if p.Enabled {
				t.Errorf("Expected groq_whisper to be disabled by default")
			}
		}
	}
	if !foundGroq {
		t.Fatalf("groq_whisper plugin not found in default registry")
	}

	// 2. Toggle plugin ON
	if err := mgr.Toggle("groq_whisper", true); err != nil {
		t.Fatalf("Failed to toggle plugin: %v", err)
	}
	if !mgr.IsEnabled("groq_whisper") {
		t.Errorf("Expected groq_whisper to be enabled after Toggle(true)")
	}

	// 3. Update configuration
	testApiKey := "gsk_test1234567890abcdef"
	newCfg := map[string]any{
		"api_key":         testApiKey,
		"model":           "whisper-large-v3-turbo",
		"language":        "es",
		"with_timestamps": true,
	}
	if err := mgr.UpdateConfig("groq_whisper", newCfg); err != nil {
		t.Fatalf("Failed to update plugin config: %v", err)
	}

	rawCfg := mgr.GetConfig("groq_whisper")
	if rawCfg["api_key"] != testApiKey {
		t.Errorf("Expected raw api_key to match %s, got %v", testApiKey, rawCfg["api_key"])
	}

	// 4. Test masking in ListPlugins(true)
	maskedPlugins := mgr.ListPlugins(true)
	for _, p := range maskedPlugins {
		if p.ID == "groq_whisper" {
			key, _ := p.Config["api_key"].(string)
			if key == testApiKey {
				t.Errorf("API key should be masked in ListPlugins(true)")
			}
			if !p.Config["has_api_key"].(bool) {
				t.Errorf("Expected has_api_key to be true")
			}
		}
	}

	// 5. Test persistence (reload in new manager instance)
	mgr2 := NewManager(tempDir)
	if !mgr2.IsEnabled("groq_whisper") {
		t.Errorf("Expected groq_whisper state to persist across manager instances")
	}
	rawCfg2 := mgr2.GetConfig("groq_whisper")
	if rawCfg2["api_key"] != testApiKey {
		t.Errorf("Expected persisted api_key to match %s, got %v", testApiKey, rawCfg2["api_key"])
	}

	// 6. Verify file exists
	if _, err := os.Stat(filepath.Join(tempDir, "plugins.json")); os.IsNotExist(err) {
		t.Errorf("plugins.json file was not created on disk")
	}
}

func TestFormatTimestamp(t *testing.T) {
	cases := []struct {
		sec      float64
		expected string
	}{
		{0.0, "00:00"},
		{5.4, "00:05"},
		{65.0, "01:05"},
		{3665.0, "01:01:05"},
	}

	for _, c := range cases {
		got := formatTimestamp(c.sec)
		if got != c.expected {
			t.Errorf("formatTimestamp(%.1f) = %s, expected %s", c.sec, got, c.expected)
		}
	}
}
