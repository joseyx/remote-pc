package obsidian

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type VaultInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	Open bool   `json:"open"`
}

type NoteSummary struct {
	Title    string    `json:"title"`
	Relative string    `json:"relative"` // relative path inside vault
	FullPath string    `json:"fullPath"`
	Modified time.Time `json:"modified"`
	Size     int64     `json:"size"`
}

type NoteDetail struct {
	Title    string    `json:"title"`
	Relative string    `json:"relative"`
	FullPath string    `json:"fullPath"`
	Content  string    `json:"content"`
	Modified time.Time `json:"modified"`
}

type Manager struct {
	knownVaults map[string]string // alias -> absolute path (e.g. "aditum" -> "C:\Users\Jose\Documents\Vaultsa\Aditum")
}

func NewManager() *Manager {
	m := &Manager{
		knownVaults: make(map[string]string),
	}
	m.discoverVaults()
	return m
}

func (m *Manager) discoverVaults() {
	var candidatePaths []string
	if appData := os.Getenv("APPDATA"); appData != "" {
		candidatePaths = append(candidatePaths, filepath.Join(appData, "obsidian", "obsidian.json"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidatePaths = append(candidatePaths,
			filepath.Join(home, "Library", "Application Support", "obsidian", "obsidian.json"),
			filepath.Join(home, ".config", "obsidian", "obsidian.json"),
			filepath.Join(home, "AppData", "Roaming", "obsidian", "obsidian.json"),
		)
	}

	var data []byte
	for _, cp := range candidatePaths {
		if d, err := os.ReadFile(cp); err == nil {
			data = d
			break
		}
	}
	if len(data) == 0 {
		return
	}

	var parsed struct {
		Vaults map[string]struct {
			Path string `json:"path"`
			Open bool   `json:"open"`
		} `json:"vaults"`
	}

	if err := json.Unmarshal(data, &parsed); err != nil {
		return
	}

	for _, v := range parsed.Vaults {
		baseName := filepath.Base(v.Path)
		cleanPath := filepath.Clean(v.Path)
		m.knownVaults[strings.ToLower(baseName)] = cleanPath
		// Also register common aliases
		if strings.Contains(strings.ToLower(cleanPath), "aditum") {
			m.knownVaults["aditum"] = cleanPath
		}
		if strings.Contains(strings.ToLower(cleanPath), "brif") {
			m.knownVaults["brif"] = cleanPath
		}
	}
}

func (m *Manager) GetVaultPath(aliasOrName string) string {
	lower := strings.ToLower(aliasOrName)
	if p, ok := m.knownVaults[lower]; ok {
		return p
	}
	// Fallback to standard paths if obsidian.json didn't have it
	home, _ := os.UserHomeDir()
	switch lower {
	case "aditum":
		if home != "" {
			candidates := []string{
				filepath.Join(home, "Documents", "Vaultsa", "Aditum"),
				filepath.Join(home, "Documents", "Vaults", "Aditum"),
				filepath.Join(home, "Vaults", "Aditum"),
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					return c
				}
			}
		}
		return `C:\Users\Jose\Documents\Vaultsa\Aditum`
	case "brif":
		if home != "" {
			candidates := []string{
				filepath.Join(home, "Documents", "Vaultsa", "Brif", "Brif"),
				filepath.Join(home, "Documents", "Vaults", "Brif"),
				filepath.Join(home, "Vaults", "Brif"),
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					return c
				}
			}
		}
		return `C:\Users\Jose\Documents\Vaultsa\Brif\Brif`
	}
	return ""
}

func (m *Manager) ListVaults() []VaultInfo {
	res := []VaultInfo{
		{
			ID:   "aditum",
			Name: "Aditum Vault",
			Path: m.GetVaultPath("aditum"),
			Open: true,
		},
		{
			ID:   "brif",
			Name: "Brif Vault",
			Path: m.GetVaultPath("brif"),
			Open: true,
		},
	}
	return res
}

func (m *Manager) SearchNotes(vaultAlias string, query string) ([]NoteSummary, error) {
	vaultPath := m.GetVaultPath(vaultAlias)
	if vaultPath == "" {
		return nil, fmt.Errorf("vault '%s' not found", vaultAlias)
	}

	var results []NoteSummary
	q := strings.ToLower(query)

	err := filepath.Walk(vaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		rel, _ := filepath.Rel(vaultPath, path)
		cleanRel := filepath.ToSlash(rel)
		fileName := strings.TrimSuffix(info.Name(), ".md")

		// Match in file name or relative path
		matched := false
		if q == "" || strings.Contains(strings.ToLower(cleanRel), q) || strings.Contains(strings.ToLower(fileName), q) {
			matched = true
		} else {
			// Search content if query is at least 3 chars
			if len(q) >= 3 {
				content, readErr := os.ReadFile(path)
				if readErr == nil && strings.Contains(strings.ToLower(string(content)), q) {
					matched = true
				}
			}
		}

		if matched {
			results = append(results, NoteSummary{
				Title:    fileName,
				Relative: cleanRel,
				FullPath: path,
				Modified: info.ModTime(),
				Size:     info.Size(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by modification time descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Modified.After(results[j].Modified)
	})

	if len(results) > 50 {
		results = results[:50]
	}

	return results, nil
}

func (m *Manager) ReadNote(vaultAlias string, relativePath string) (*NoteDetail, error) {
	vaultPath := m.GetVaultPath(vaultAlias)
	if vaultPath == "" {
		return nil, fmt.Errorf("vault '%s' not found", vaultAlias)
	}

	fullPath := filepath.Join(vaultPath, filepath.FromSlash(relativePath))
	// Prevent path traversal
	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(vaultPath)) {
		return nil, fmt.Errorf("invalid path traversal")
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	return &NoteDetail{
		Title:    strings.TrimSuffix(filepath.Base(fullPath), ".md"),
		Relative: filepath.ToSlash(relativePath),
		FullPath: fullPath,
		Content:  string(data),
		Modified: info.ModTime(),
	}, nil
}

func (m *Manager) SaveNote(vaultAlias string, relativePath string, content string) error {
	vaultPath := m.GetVaultPath(vaultAlias)
	if vaultPath == "" {
		return fmt.Errorf("vault '%s' not found", vaultAlias)
	}

	if !strings.HasSuffix(strings.ToLower(relativePath), ".md") {
		relativePath += ".md"
	}

	fullPath := filepath.Join(vaultPath, filepath.FromSlash(relativePath))
	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(vaultPath)) {
		return fmt.Errorf("invalid path traversal")
	}

	// Ensure parent dir exists
	parent := filepath.Dir(fullPath)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}

	return os.WriteFile(fullPath, []byte(content), 0644)
}
