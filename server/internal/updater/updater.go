package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	GitHubOwner = "joseyx"
	GitHubRepo  = "remote-pc"
	ApiURL      = "https://api.github.com/repos/" + GitHubOwner + "/" + GitHubRepo + "/releases/latest"
	UserAgent   = "Orbit-Updater"
)

// ReleaseInfo holds details about the latest available release and whether an update is needed.
type ReleaseInfo struct {
	CurrentVersion   string `json:"currentVersion"`
	LatestVersion    string `json:"latestVersion"`
	UpdateAvailable  bool   `json:"updateAvailable"`
	ReleaseName      string `json:"releaseName"`
	Changelog        string `json:"changelog"`
	ReleaseURL       string `json:"releaseUrl"`
	PublishedAt      string `json:"publishedAt"`
	AssetDownloadURL string `json:"assetDownloadUrl"`
	AssetName        string `json:"assetName"`
	AssetSize        int64  `json:"assetSize"`
	InstallerURL     string `json:"installerUrl,omitempty"`
}

type ghReleaseResponse struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckUpdate queries GitHub for the latest release and compares versions.
func CheckUpdate(currentVersion string) (*ReleaseInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", ApiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating update request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error connecting to update server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status checking updates: %s", resp.Status)
	}

	var ghRel ghReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&ghRel); err != nil {
		return nil, fmt.Errorf("error decoding release metadata: %w", err)
	}

	latestVer := strings.TrimPrefix(ghRel.TagName, "v")
	currentClean := strings.TrimPrefix(currentVersion, "v")
	cmp := CompareVersions(currentClean, latestVer)
	updateAvailable := cmp < 0

	// Select matching asset based on OS and ARCH
	targetAssetName := getExpectedAssetName(runtime.GOOS, runtime.GOARCH)
	var matchedAsset *ghAsset
	var installerAsset *ghAsset

	for i := range ghRel.Assets {
		asset := &ghRel.Assets[i]
		if asset.Name == targetAssetName {
			matchedAsset = asset
		}
		if asset.Name == "Orbit-Setup.exe" {
			installerAsset = asset
		}
	}

	info := &ReleaseInfo{
		CurrentVersion:  currentVersion,
		LatestVersion:   latestVer,
		UpdateAvailable: updateAvailable,
		ReleaseName:     ghRel.Name,
		Changelog:       ghRel.Body,
		ReleaseURL:      ghRel.HTMLURL,
		PublishedAt:     ghRel.PublishedAt.Format(time.RFC3339),
	}

	if matchedAsset != nil {
		info.AssetDownloadURL = matchedAsset.BrowserDownloadURL
		info.AssetName = matchedAsset.Name
		info.AssetSize = matchedAsset.Size
	}
	if installerAsset != nil {
		info.InstallerURL = installerAsset.BrowserDownloadURL
	}

	return info, nil
}

// ApplyUpdate downloads the release asset and updates the running binary in-place.
func ApplyUpdate(info *ReleaseInfo) error {
	if info.AssetDownloadURL == "" {
		return fmt.Errorf("no matching release binary asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// 1. Download asset into memory/temp buffer
	data, err := downloadAsset(info.AssetDownloadURL)
	if err != nil {
		return fmt.Errorf("error downloading update asset: %w", err)
	}

	// 2. Extract and update binary
	exeDir := filepath.Dir(exePath)
	if runtime.GOOS == "windows" {
		return applyWindowsUpdate(exePath, exeDir, data)
	}
	return applyUnixUpdate(exePath, exeDir, data)
}

func downloadAsset(url string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with HTTP %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func applyWindowsUpdate(exePath, exeDir string, zipData []byte) error {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("invalid zip archive: %w", err)
	}

	var orbitBinary []byte
	var remotepcBinary []byte

	for _, f := range reader.File {
		name := filepath.Base(f.Name)
		if strings.EqualFold(name, "orbit.exe") {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			orbitBinary, _ = io.ReadAll(rc)
			_ = rc.Close()
		} else if strings.EqualFold(name, "remotepc-server.exe") {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			remotepcBinary, _ = io.ReadAll(rc)
			_ = rc.Close()
		}
	}

	if len(orbitBinary) == 0 {
		return fmt.Errorf("archive does not contain orbit.exe")
	}

	// On Windows, rename running executable to .old
	oldExe := exePath + ".old"
	_ = os.Remove(oldExe)
	if err := os.Rename(exePath, oldExe); err != nil {
		return fmt.Errorf("failed to move current executable (permission denied?): %w", err)
	}

	// Write new binary
	if err := os.WriteFile(exePath, orbitBinary, 0755); err != nil {
		// Rollback on failure
		_ = os.Rename(oldExe, exePath)
		return fmt.Errorf("failed to write new executable: %w", err)
	}

	// Also update remotepc-server.exe if present
	remotepcPath := filepath.Join(exeDir, "remotepc-server.exe")
	if _, err := os.Stat(remotepcPath); err == nil {
		oldRemotePC := remotepcPath + ".old"
		_ = os.Remove(oldRemotePC)
		if err := os.Rename(remotepcPath, oldRemotePC); err == nil {
			if len(remotepcBinary) > 0 {
				_ = os.WriteFile(remotepcPath, remotepcBinary, 0755)
			} else {
				_ = os.WriteFile(remotepcPath, orbitBinary, 0755)
			}
		}
	}

	// Cleanup old binary if possible, ignore error if locked until process exit
	_ = os.Remove(oldExe)

	return nil
}

func applyUnixUpdate(exePath, exeDir string, tarGzData []byte) error {
	gzr, err := gzip.NewReader(bytes.NewReader(tarGzData))
	if err != nil {
		return fmt.Errorf("invalid gzip archive: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var newBinary []byte

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar archive: %w", err)
		}

		baseName := filepath.Base(header.Name)
		if baseName == "orbit" && (header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA) {
			newBinary, err = io.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("error reading binary from archive: %w", err)
			}
			break
		}
	}

	if len(newBinary) == 0 {
		return fmt.Errorf("archive does not contain orbit executable")
	}

	// Write to temporary file in the same directory (guarantees same filesystem for atomic rename)
	tempFile, err := os.CreateTemp(exeDir, ".orbit-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file for update: %w", err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(newBinary); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to write updated binary: %w", err)
	}
	_ = tempFile.Close()

	if err := os.Chmod(tempPath, 0755); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	// Atomic replacement
	if err := os.Rename(tempPath, exePath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to replace executable: %w", err)
	}

	// If remotepc-server exists in exeDir, update it as well
	remotepcPath := filepath.Join(exeDir, "remotepc-server")
	if fi, err := os.Lstat(remotepcPath); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			// Symlink: ensure it points to orbit
			_ = os.Remove(remotepcPath)
			_ = os.Symlink(filepath.Base(exePath), remotepcPath)
		} else {
			// Regular file: duplicate binary
			_ = os.WriteFile(remotepcPath, newBinary, 0755)
		}
	}

	return nil
}

// CleanOldBinaries removes leftover .old backup files from previous updates.
func CleanOldBinaries(exePath string) {
	oldExe := exePath + ".old"
	_ = os.Remove(oldExe)

	exeDir := filepath.Dir(exePath)
	oldRemotePC := filepath.Join(exeDir, "remotepc-server.exe.old")
	_ = os.Remove(oldRemotePC)
}

func getExpectedAssetName(goos, goarch string) string {
	switch goos {
	case "windows":
		return fmt.Sprintf("orbit-windows-%s.zip", goarch)
	case "linux":
		return fmt.Sprintf("orbit-linux-%s.tar.gz", goarch)
	case "darwin":
		return fmt.Sprintf("orbit-darwin-%s.tar.gz", goarch)
	default:
		return fmt.Sprintf("orbit-%s-%s.tar.gz", goos, goarch)
	}
}

// CompareVersions compares two semver strings (e.g. "1.0.0" and "1.1.0").
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
func CompareVersions(v1, v2 string) int {
	clean1 := cleanSemver(v1)
	clean2 := cleanSemver(v2)

	nums1 := parseVersionNumbers(clean1)
	nums2 := parseVersionNumbers(clean2)

	for i := 0; i < 3; i++ {
		if nums1[i] < nums2[i] {
			return -1
		}
		if nums1[i] > nums2[i] {
			return 1
		}
	}

	return 0
}

func cleanSemver(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	// Strip prerelease or build metadata if present (e.g. 1.0.0-rc1 -> 1.0.0)
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		v = v[:idx]
	}
	return v
}

func parseVersionNumbers(v string) [3]int {
	var result [3]int
	parts := strings.Split(v, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		num, err := strconv.Atoi(parts[i])
		if err == nil {
			result[i] = num
		}
	}
	return result
}
