package updater

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.0.0", "1.0.0", 0},
		{"v1.0.0", "1.0.0", 0},
		{"1.0.0", "v1.0.1", -1},
		{"1.1.0", "1.0.9", 1},
		{"2.0.0", "1.99.99", 1},
		{"0.9.5", "1.0.0", -1},
		{"v1.0.0-rc1", "1.0.0", 0},
		{"1.0.1-beta.1", "1.0.0", 1},
		{"v1.2.3", "v1.2.4", -1},
	}

	for _, tt := range tests {
		res := CompareVersions(tt.v1, tt.v2)
		if res != tt.expected {
			t.Errorf("CompareVersions(%q, %q) = %d; want %d", tt.v1, tt.v2, res, tt.expected)
		}
	}
}

func TestGetExpectedAssetName(t *testing.T) {
	if got := getExpectedAssetName("windows", "amd64"); got != "orbit-windows-amd64.zip" {
		t.Errorf("windows asset name = %s; want orbit-windows-amd64.zip", got)
	}
	if got := getExpectedAssetName("linux", "amd64"); got != "orbit-linux-amd64.tar.gz" {
		t.Errorf("linux amd64 asset name = %s; want orbit-linux-amd64.tar.gz", got)
	}
	if got := getExpectedAssetName("darwin", "arm64"); got != "orbit-darwin-arm64.tar.gz" {
		t.Errorf("darwin arm64 asset name = %s; want orbit-darwin-arm64.tar.gz", got)
	}
}

func TestCheckUpdateLive(t *testing.T) {
	info, err := CheckUpdate("0.9.0")
	if err != nil {
		t.Skipf("Network/GitHub API unreachable: %v", err)
	}
	if info.LatestVersion == "" {
		t.Errorf("expected non-empty LatestVersion")
	}
	if !info.UpdateAvailable {
		t.Errorf("expected UpdateAvailable to be true for v0.9.0 when latest is %s", info.LatestVersion)
	}
	if info.AssetDownloadURL == "" {
		t.Errorf("expected non-empty AssetDownloadURL for current OS/arch")
	}
}

