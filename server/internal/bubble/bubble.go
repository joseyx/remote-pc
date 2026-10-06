package bubble

import (
	"time"
)

type WorkspaceType string

const (
	WorkspaceAditum  WorkspaceType = "aditum"
	WorkspaceBrif    WorkspaceType = "brif"
	WorkspaceGeneral WorkspaceType = "general"
)

type BubbleStatus string

const (
	StatusIdle     BubbleStatus = "idle"
	StatusRunning  BubbleStatus = "running"
	StatusStarting BubbleStatus = "starting"
	StatusError    BubbleStatus = "error"
)

type Bubble struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Workspace  WorkspaceType     `json:"workspace"`
	Path       string            `json:"path"`
	Branch     string            `json:"branch"`
	BaseBranch string            `json:"baseBranch,omitempty"`
	Port       int               `json:"port"`
	DbSchema   string            `json:"dbSchema,omitempty"`
	Status     BubbleStatus      `json:"status"`
	CreatedAt  time.Time         `json:"createdAt"`
	IsWorktree bool              `json:"isWorktree"`
	DevRunning bool              `json:"devRunning"`
	DevPid     int               `json:"devPid,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type WorkspaceInfo struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        WorkspaceType `json:"type"`
	BasePath    string        `json:"basePath"`
	VaultAlias  string        `json:"vaultAlias"`
	DefaultBase string        `json:"defaultBase"`
	Description string        `json:"description"`
}

type CreateBubbleRequest struct {
	Workspace  WorkspaceType `json:"workspace"`
	TaskID     string        `json:"taskId"`     // e.g. "TAR-1120" (Aditum) or "dialogos-mx" (Brif)
	Name       string        `json:"name"`       // optional human name
	BaseBranch string        `json:"baseBranch"` // e.g. "development"
	UseSlim    bool          `json:"useSlim"`    // For Aditum: seed aditum_slim (true by default)
	SeedSchema string        `json:"seedSchema"` // Custom seed schema if any
	Index      int           `json:"index"`      // Port index 1-99 for Aditum
	IsWorktree bool          `json:"isWorktree"` // For Brif: true if isolated worktree, false if in-place branch
}
