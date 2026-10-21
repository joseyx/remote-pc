package plugin

// ConfigField defines a configurable parameter for a plugin
type ConfigField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"` // "text", "password", "select", "checkbox", "number"
	Options     []string `json:"options,omitempty"`
	Default     any      `json:"default,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Help        string   `json:"help,omitempty"`
	Required    bool     `json:"required,omitempty"`
}

// PluginInfo represents the metadata, status and configuration of an Orbit plugin
type PluginInfo struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Icon            string         `json:"icon"`
	Category        string         `json:"category"`
	Version         string         `json:"version"`
	Author          string         `json:"author"`
	Enabled         bool           `json:"enabled"`
	RequiresRestart bool           `json:"requiresRestart"`
	Config          map[string]any `json:"config"`
	ConfigSchema    []ConfigField  `json:"configSchema,omitempty"`
}

// Segment represents a timed snippet in a Whisper transcript
type Segment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// TranscriptionResult represents the structured result of an audio/video transcription
type TranscriptionResult struct {
	Text          string    `json:"text"`
	FormattedText string    `json:"formattedText"`
	Language      string    `json:"language"`
	Duration      float64   `json:"duration"`
	Segments      []Segment `json:"segments,omitempty"`
}
