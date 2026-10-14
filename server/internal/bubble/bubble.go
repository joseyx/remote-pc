package bubble

import (
	"time"
)

type WorkspaceType string

const (
	WorkspaceAditum  WorkspaceType = "aditum"
	WorkspaceBrif    WorkspaceType = "brif"
	WorkspaceGeneral WorkspaceType = "general"
	WorkspaceGit     WorkspaceType = "git"
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
	Priority   string            `json:"priority,omitempty"`
	CustomRank int               `json:"customRank,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type WorkspaceInfo struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        WorkspaceType `json:"type"`
	BasePath    string        `json:"basePath"`
	VaultAlias  string        `json:"vaultAlias,omitempty"`
	DefaultBase string        `json:"defaultBase,omitempty"`
	Description string        `json:"description,omitempty"`
	DevCommand  string        `json:"devCommand,omitempty"`
	DefaultPort int           `json:"defaultPort,omitempty"`
	RepoURL     string        `json:"repoUrl,omitempty"`
}

type BranchInfo struct {
	Name     string `json:"name"`
	IsRemote bool   `json:"isRemote"`
}

type SqlDumpInfo struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	SizeStr  string    `json:"sizeStr"`
	ModTime  time.Time `json:"modTime"`
}

type CreateBubbleRequest struct {
	Workspace   WorkspaceType `json:"workspace"`
	TaskID      string        `json:"taskId"`                // e.g. "TAR-1120" (Aditum), "dialogos-mx" (Brif), or "feature-login"
	Name        string        `json:"name"`                  // optional human name
	BaseBranch  string        `json:"baseBranch"`            // e.g. "development", "main"
	CloneBranch string        `json:"cloneBranch,omitempty"` // If set, checkout/clone this existing branch directly
	UseSlim     bool          `json:"useSlim"`               // For Aditum: seed aditum_slim (true by default)
	SeedSchema  string        `json:"seedSchema,omitempty"`  // Custom seed schema if any
	SeedDump    string        `json:"seedDump,omitempty"`    // Custom SQL dump file path if any
	Index       int           `json:"index"`                 // Port index 1-99 for Aditum
	IsWorktree  bool          `json:"isWorktree"`            // true if isolated worktree, false if in-place branch
	Port        int           `json:"port,omitempty"`        // Custom port override
}
