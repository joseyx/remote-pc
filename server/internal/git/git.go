package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type FileChange struct {
	Path   string `json:"path"`
	Status string `json:"status"` // "M", "A", "D", "R", "??"
	Staged bool   `json:"staged"`
}

type StatusResult struct {
	Branch  string       `json:"branch"`
	IsDirty bool         `json:"isDirty"`
	Ahead   int          `json:"ahead"`
	Behind  int          `json:"behind"`
	Changes []FileChange `json:"changes"`
}

type CommitInfo struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

func isGitRepo(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info != nil
}

func getSubGitRepos(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var repos []string
	for _, e := range entries {
		if e.IsDir() {
			sub := filepath.Join(dir, e.Name())
			if isGitRepo(sub) {
				repos = append(repos, e.Name())
			}
		}
	}
	return repos
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(errOut.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func GetBranch(dir string) (string, error) {
	if !isGitRepo(dir) {
		subRepos := getSubGitRepos(dir)
		if len(subRepos) == 0 {
			return "monorepo", nil
		}
		primary := subRepos[0]
		for _, sub := range subRepos {
			if strings.Contains(sub, "scrapper") {
				primary = sub
				break
			}
		}
		return GetBranch(filepath.Join(dir, primary))
	}
	return runGit(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

func GetStatus(dir string) (*StatusResult, error) {
	if !isGitRepo(dir) {
		subRepos := getSubGitRepos(dir)
		if len(subRepos) == 0 {
			return &StatusResult{Branch: "none", Changes: []FileChange{}}, nil
		}

		res := &StatusResult{
			Branch:  "",
			Changes: make([]FileChange, 0),
		}

		var branchParts []string
		for _, sub := range subRepos {
			subDir := filepath.Join(dir, sub)
			subStatus, err := GetStatus(subDir)
			if err != nil {
				continue
			}
			if subStatus.Branch != "" {
				branchParts = append(branchParts, fmt.Sprintf("%s:%s", sub, subStatus.Branch))
			}
			if subStatus.IsDirty {
				res.IsDirty = true
			}
			res.Ahead += subStatus.Ahead
			res.Behind += subStatus.Behind
			for _, ch := range subStatus.Changes {
				ch.Path = filepath.ToSlash(filepath.Join(sub, ch.Path))
				res.Changes = append(res.Changes, ch)
			}
		}

		if len(branchParts) > 0 {
			res.Branch = strings.Join(branchParts, " | ")
		} else {
			res.Branch = "monorepo"
		}
		return res, nil
	}

	branch, _ := GetBranch(dir)

	out, err := runGit(dir, "status", "--porcelain=v1", "-b")
	if err != nil {
		return nil, err
	}

	res := &StatusResult{
		Branch:  branch,
		Changes: make([]FileChange, 0),
	}

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if strings.Contains(line, "[ahead ") {
				var a int
				if _, scanErr := fmt.Sscanf(line[strings.Index(line, "[ahead "):], "[ahead %d", &a); scanErr == nil {
					res.Ahead = a
				}
			}
			if strings.Contains(line, "behind ") {
				var b int
				if _, scanErr := fmt.Sscanf(line[strings.Index(line, "behind "):], "behind %d", &b); scanErr == nil {
					res.Behind = b
				}
			}
			continue
		}

		if len(line) < 3 {
			continue
		}

		x := line[0] // index status
		y := line[1] // work tree status
		path := strings.TrimSpace(line[3:])

		res.IsDirty = true

		if x != ' ' && x != '?' {
			res.Changes = append(res.Changes, FileChange{
				Path:   path,
				Status: string(x),
				Staged: true,
			})
		}
		if y != ' ' {
			st := string(y)
			if x == '?' && y == '?' {
				st = "??"
			}
			res.Changes = append(res.Changes, FileChange{
				Path:   path,
				Status: st,
				Staged: false,
			})
		}
	}

	return res, nil
}

func GetDiff(dir string, filePath string) (string, error) {
	if !isGitRepo(dir) {
		subRepos := getSubGitRepos(dir)
		if len(subRepos) == 0 {
			return "", nil
		}

		if filePath != "" {
			normFile := filepath.ToSlash(filePath)
			for _, sub := range subRepos {
				prefix := sub + "/"
				if strings.HasPrefix(normFile, prefix) {
					relPath := strings.TrimPrefix(normFile, prefix)
					return GetDiff(filepath.Join(dir, sub), relPath)
				}
			}
		}

		// Combined diff
		var combined bytes.Buffer
		for _, sub := range subRepos {
			diff, err := GetDiff(filepath.Join(dir, sub), "")
			if err == nil && strings.TrimSpace(diff) != "" {
				combined.WriteString(fmt.Sprintf("=== [%s] ===\n%s\n\n", sub, diff))
			}
		}
		return combined.String(), nil
	}

	if filePath != "" {
		return runGit(dir, "diff", "HEAD", "--", filePath)
	}
	return runGit(dir, "diff", "HEAD")
}

func GetRecentLog(dir string, count int) ([]CommitInfo, error) {
	if !isGitRepo(dir) {
		subRepos := getSubGitRepos(dir)
		if len(subRepos) == 0 {
			return nil, nil
		}
		primary := subRepos[0]
		for _, sub := range subRepos {
			if strings.Contains(sub, "scrapper") {
				primary = sub
				break
			}
		}
		return GetRecentLog(filepath.Join(dir, primary), count)
	}

	if count <= 0 {
		count = 15
	}
	format := "%h%x09%an%x09%ad%x09%s"
	out, err := runGit(dir, "log", fmt.Sprintf("-n%d", count), fmt.Sprintf("--format=%s", format), "--date=relative")
	if err != nil {
		return nil, err
	}

	var commits []CommitInfo
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) >= 4 {
			commits = append(commits, CommitInfo{
				Hash:    parts[0],
				Author:  parts[1],
				Date:    parts[2],
				Message: parts[3],
			})
		}
	}
	return commits, nil
}

func ListBranches(dir string) ([]string, error) {
	if !isGitRepo(dir) {
		subRepos := getSubGitRepos(dir)
		if len(subRepos) == 0 {
			return nil, nil
		}
		primary := subRepos[0]
		for _, sub := range subRepos {
			if strings.Contains(sub, "scrapper") {
				primary = sub
				break
			}
		}
		return ListBranches(filepath.Join(dir, primary))
	}

	out, err := runGit(dir, "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, b := range strings.Split(out, "\n") {
		b = strings.TrimSpace(b)
		if b != "" {
			branches = append(branches, b)
		}
	}
	return branches, nil
}

func CheckoutBranch(dir string, branch string, create bool) error {
	if !isGitRepo(dir) {
		subRepos := getSubGitRepos(dir)
		if len(subRepos) == 0 {
			return fmt.Errorf("not a git repository")
		}
		primary := subRepos[0]
		for _, sub := range subRepos {
			if strings.Contains(sub, "scrapper") {
				primary = sub
				break
			}
		}
		return CheckoutBranch(filepath.Join(dir, primary), branch, create)
	}

	if create {
		_, err := runGit(dir, "checkout", "-b", branch)
		return err
	}
	_, err := runGit(dir, "checkout", branch)
	return err
}
