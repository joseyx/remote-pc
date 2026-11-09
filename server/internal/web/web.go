package web

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

//go:embed static/*
var staticFS embed.FS

// Handler returns an http.Handler that serves the web dashboard.
// If an override static folder exists on disk (via ORBIT_STATIC_DIR, dataDir/static or LOCALAPPDATA/Orbit/static),
// it serves from disk to enable instant hot-reload of UI without restarting the binary.
// Otherwise, it falls back to the embedded staticFS.
func Handler(dataDir ...string) http.Handler {
	var fileSystem http.FileSystem

	var overrideDir string
	if envDir := os.Getenv("ORBIT_STATIC_DIR"); envDir != "" {
		overrideDir = envDir
	} else if len(dataDir) > 0 && dataDir[0] != "" {
		candidate := filepath.Join(dataDir[0], "static")
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			overrideDir = candidate
		}
	} else if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
		candidate := filepath.Join(localApp, "Orbit", "static")
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			overrideDir = candidate
		}
	}

	if overrideDir != "" {
		if _, err := os.Stat(filepath.Join(overrideDir, "index.html")); err == nil {
			fileSystem = http.Dir(overrideDir)
		}
	}

	if fileSystem == nil {
		sub, err := fs.Sub(staticFS, "static")
		if err != nil {
			panic(err)
		}
		fileSystem = http.FS(sub)
	}

	fileServer := http.FileServer(fileSystem)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fileServer.ServeHTTP(w, r)
	})
}

// ExtractStaticAssets unpacks embedded static dashboard files to the target directory on disk
func ExtractStaticAssets(targetDir string) error {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("failed to get sub fs: %w", err)
	}

	return fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		destPath := filepath.Join(targetDir, path)
		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		srcFile, err := sub.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}
