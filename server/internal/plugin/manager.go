package plugin

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type storedPluginState struct {
	Enabled bool           `json:"enabled"`
	Config  map[string]any `json:"config"`
}

type Manager struct {
	mu       sync.RWMutex
	dataDir  string
	filePath string
	states   map[string]storedPluginState
}

// Built-in registry of available plugins
var defaultRegistry = []PluginInfo{
	{
		ID:              "groq_whisper",
		Name:            "Groq Whisper Video Transcriber",
		Description:     "Extrae audio de videos (.mp4, .mkv, .mov, .webm) con FFmpeg y los transcribe a velocidad ultra-rápida (LPU) vía Groq Whisper. Expone herramientas automáticas a Claude Code y Antigravity.",
		Icon:            "🎙️",
		Category:        "IA & Transcripción",
		Version:         "1.0.0",
		Author:          "Orbit Core",
		Enabled:         false,
		RequiresRestart: true,
		Config: map[string]any{
			"api_key":         "",
			"model":           "whisper-large-v3-turbo",
			"language":        "auto",
			"with_timestamps": true,
		},
		ConfigSchema: []ConfigField{
			{
				Key:         "api_key",
				Label:       "Groq API Key",
				Type:        "password",
				Placeholder: "gsk_...",
				Help:        "Obtén tu API key gratuita en https://console.groq.com/keys",
				Required:    true,
			},
			{
				Key:     "model",
				Label:   "Modelo de Whisper",
				Type:    "select",
				Options: []string{"whisper-large-v3-turbo", "whisper-large-v3"},
				Default: "whisper-large-v3-turbo",
				Help:    "whisper-large-v3-turbo es ~5x más rápido y económico con calidad sobresaliente",
			},
			{
				Key:     "language",
				Label:   "Idioma base",
				Type:    "select",
				Options: []string{"auto", "es", "en", "pt", "fr", "de", "it"},
				Default: "auto",
				Help:    "auto detecta automáticamente el idioma hablado en la grabación",
			},
			{
				Key:     "with_timestamps",
				Label:   "Incluir marcas de tiempo por defecto",
				Type:    "checkbox",
				Default: true,
				Help:    "Añade marcas de tiempo [MM:SS - MM:SS] en los párrafos transcritos",
			},
		},
	},
}

// Global default manager instance for MCP server access
var (
	globalManager *Manager
	globalMu      sync.RWMutex
)

func SetGlobalManager(m *Manager) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalManager = m
}

func GetGlobalManager() *Manager {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalManager
}

func NewManager(dataDir string) *Manager {
	m := &Manager{
		dataDir:  dataDir,
		filePath: filepath.Join(dataDir, "plugins.json"),
		states:   make(map[string]storedPluginState),
	}
	m.load()
	SetGlobalManager(m)
	return m
}

func (m *Manager) load() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err == nil {
		if err := json.Unmarshal(data, &m.states); err != nil {
			log.Printf("[plugins] advertencia: error al parsear %s: %v", m.filePath, err)
		}
	}

	if m.states == nil {
		m.states = make(map[string]storedPluginState)
	}

	for id, st := range m.states {
		if st.Config == nil {
			st.Config = make(map[string]any)
			m.states[id] = st
		}
	}
}

// Reload re-reads stored plugin states from disk
func (m *Manager) Reload() {
	m.load()
}

func (m *Manager) saveLocked() error {
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return fmt.Errorf("error al crear directorio de datos de plugins: %w", err)
	}
	data, err := json.MarshalIndent(m.states, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0600)
}

// ListPlugins returns all plugins, optionally masking sensitive fields like api_key
func (m *Manager) ListPlugins(maskSecrets bool) []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]PluginInfo, 0, len(defaultRegistry))
	for _, def := range defaultRegistry {
		p := def
		// Deep copy config map
		p.Config = make(map[string]any)
		for k, v := range def.Config {
			p.Config[k] = v
		}

		if st, ok := m.states[p.ID]; ok {
			p.Enabled = st.Enabled
			for k, v := range st.Config {
				p.Config[k] = v
			}
		}

		if p.ID == "groq_whisper" {
			if apiKey, _ := p.Config["api_key"].(string); strings.TrimSpace(apiKey) == "" {
				if envKey := os.Getenv("GROQ_API_KEY"); envKey != "" {
					p.Config["api_key"] = envKey
				}
			}
		}

		if maskSecrets {
			if key, ok := p.Config["api_key"].(string); ok && key != "" {
				if len(key) > 8 {
					p.Config["api_key"] = key[:4] + "••••••••" + key[len(key)-4:]
					p.Config["has_api_key"] = true
				} else {
					p.Config["api_key"] = "••••••••"
					p.Config["has_api_key"] = true
				}
			} else {
				p.Config["has_api_key"] = false
			}
		}

		result = append(result, p)
	}
	return result
}

// GetPlugin retrieves a single plugin info
func (m *Manager) GetPlugin(id string, maskSecrets bool) (*PluginInfo, error) {
	list := m.ListPlugins(maskSecrets)
	for _, p := range list {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("plugin no encontrado: %s", id)
}

// IsEnabled checks if a specific plugin is enabled
func (m *Manager) IsEnabled(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if st, ok := m.states[id]; ok {
		return st.Enabled
	}
	return false
}

// GetConfig returns the raw config map of a plugin (unmasked)
func (m *Manager) GetConfig(id string) map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg := make(map[string]any)
	for _, def := range defaultRegistry {
		if def.ID == id {
			for k, v := range def.Config {
				cfg[k] = v
			}
			break
		}
	}

	if st, ok := m.states[id]; ok {
		for k, v := range st.Config {
			cfg[k] = v
		}
	}

	// Fallback to env variable if api_key is empty
	if id == "groq_whisper" {
		if apiKey, _ := cfg["api_key"].(string); strings.TrimSpace(apiKey) == "" {
			if envKey := os.Getenv("GROQ_API_KEY"); envKey != "" {
				cfg["api_key"] = envKey
			}
		}
	}

	return cfg
}

// Toggle changes the enabled status of a plugin
func (m *Manager) Toggle(id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	st, ok := m.states[id]
	if !ok {
		// Initialize from defaults
		st = storedPluginState{
			Enabled: enabled,
			Config:  make(map[string]any),
		}
		for _, def := range defaultRegistry {
			if def.ID == id {
				for k, v := range def.Config {
					st.Config[k] = v
				}
				break
			}
		}
	} else {
		st.Enabled = enabled
	}

	m.states[id] = st
	return m.saveLocked()
}

// UpdateConfig updates configuration settings for a plugin
func (m *Manager) UpdateConfig(id string, newConfig map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	st, ok := m.states[id]
	if !ok {
		st = storedPluginState{
			Enabled: false,
			Config:  make(map[string]any),
		}
		for _, def := range defaultRegistry {
			if def.ID == id {
				for k, v := range def.Config {
					st.Config[k] = v
				}
				break
			}
		}
	}

	for k, v := range newConfig {
		// If api_key was masked in UI (contains bullet points) and not changed, don't overwrite
		if k == "api_key" {
			if strVal, ok := v.(string); ok {
				if strings.Contains(strVal, "••") {
					continue
				}
			}
		}
		st.Config[k] = v
	}

	m.states[id] = st
	return m.saveLocked()
}
