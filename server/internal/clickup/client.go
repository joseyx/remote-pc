// Package clickup brings the ClickUp comments addressed to the user into Orbit: mentions, replies in
// threads they took part in and comments on tickets assigned to them. A background poller in the
// daemon reads the public API v2, keeps its own store and drops each new item into the daily note.
package clickup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBaseURL is the public ClickUp API v2.
	DefaultBaseURL = "https://api.clickup.com/api/v2"

	requestTimeout = 20 * time.Second
	userAgent      = "Orbit-ClickUp"
)

// requestGap spaces calls so a poll stays well under ClickUp's 100 requests/minute per token.
// It is a variable only so tests against a fake server do not wait.
var requestGap = 700 * time.Millisecond

// ErrUnauthorized means ClickUp rejected the token.
var ErrUnauthorized = errors.New("ClickUp rechazó el token (401): revísalo en Orbit > Plugins > ClickUp")

// ErrNotFound means the task or comment no longer exists (or the token lost access to it).
var ErrNotFound = errors.New("ClickUp no encontró el recurso (404)")

// CommentPageSize is how many comments ClickUp returns per page of a task.
const CommentPageSize = 25

// RateLimitError means ClickUp answered 429; RetryAt is when the window resets (zero if unknown).
type RateLimitError struct{ RetryAt time.Time }

func (e *RateLimitError) Error() string {
	if e.RetryAt.IsZero() {
		return "ClickUp limitó las peticiones (429)"
	}
	return fmt.Sprintf("ClickUp limitó las peticiones (429) hasta las %s", e.RetryAt.Format("15:04:05"))
}

// User is a ClickUp member as it appears in comments, assignees and /user.
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

// Team is a ClickUp workspace.
type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Task holds the task fields the poller needs.
type Task struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DateUpdated string `json:"date_updated"`
	Assignees   []User `json:"assignees"`
}

// IsAssignedTo reports whether userID is among the task assignees.
func (t Task) IsAssignedTo(userID int64) bool {
	for _, a := range t.Assignees {
		if a.ID == userID {
			return true
		}
	}
	return false
}

// CommentPart is one segment of a comment body; mentions are parts with Type "tag".
type CommentPart struct {
	Text string `json:"text"`
	Type string `json:"type"`
	User *User  `json:"user"`
}

// Comment is a top-level task comment or a threaded reply.
type Comment struct {
	ID         string        `json:"id"`
	Parts      []CommentPart `json:"comment"`
	Text       string        `json:"comment_text"`
	User       User          `json:"user"`
	Assignee   *User         `json:"assignee"`
	Date       string        `json:"date"`
	ReplyCount int           `json:"reply_count"`
}

// Time parses the comment's millisecond timestamp; zero if it is missing or malformed.
func (c Comment) Time() time.Time {
	return parseMillis(c.Date)
}

func parseMillis(s string) time.Time {
	ms, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

// Client is a minimal, rate-spaced ClickUp API v2 client. It is safe for concurrent use.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
	gap     time.Duration

	mu   sync.Mutex
	last time.Time
}

// NewClient builds a client; an empty baseURL means the public API.
func NewClient(baseURL, token string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   strings.TrimSpace(token),
		http:    &http.Client{Timeout: requestTimeout},
		gap:     requestGap,
	}
}

// Me returns the user that owns the token.
func (c *Client) Me(ctx context.Context) (User, error) {
	var out struct {
		User User `json:"user"`
	}
	if err := c.get(ctx, "/user", nil, &out); err != nil {
		return User{}, err
	}
	if out.User.ID == 0 {
		return User{}, errors.New("ClickUp no devolvió el usuario del token")
	}
	return out.User, nil
}

// Teams lists the workspaces the token can see.
func (c *Client) Teams(ctx context.Context) ([]Team, error) {
	var out struct {
		Teams []Team `json:"teams"`
	}
	if err := c.get(ctx, "/team", nil, &out); err != nil {
		return nil, err
	}
	return out.Teams, nil
}

// TaskQuery filters a workspace task listing.
type TaskQuery struct {
	UpdatedAfter  time.Time
	AssigneeID    int64 // 0 = any
	IncludeClosed bool
	MaxPages      int
}

// Tasks lists workspace tasks updated after q.UpdatedAfter, following pagination up to q.MaxPages.
// It reports truncated=true when more pages were left unread.
func (c *Client) Tasks(ctx context.Context, teamID string, q TaskQuery) (tasks []Task, truncated bool, err error) {
	maxPages := q.MaxPages
	if maxPages <= 0 {
		maxPages = 1
	}
	for page := 0; page < maxPages; page++ {
		v := url.Values{}
		v.Set("order_by", "updated")
		v.Set("subtasks", "true")
		v.Set("page", strconv.Itoa(page))
		if q.IncludeClosed {
			v.Set("include_closed", "true")
		}
		if !q.UpdatedAfter.IsZero() {
			v.Set("date_updated_gt", strconv.FormatInt(q.UpdatedAfter.UnixMilli(), 10))
		}
		if q.AssigneeID != 0 {
			v.Set("assignees[]", strconv.FormatInt(q.AssigneeID, 10))
		}
		var out struct {
			Tasks    []Task `json:"tasks"`
			LastPage *bool  `json:"last_page"`
		}
		if err := c.get(ctx, "/team/"+url.PathEscape(teamID)+"/task", v, &out); err != nil {
			return tasks, false, err
		}
		tasks = append(tasks, out.Tasks...)
		if len(out.Tasks) == 0 || (out.LastPage != nil && *out.LastPage) {
			return tasks, false, nil
		}
	}
	return tasks, true, nil
}

// TaskComments returns the newest top-level comments of a task (ClickUp sends up to 25).
func (c *Client) TaskComments(ctx context.Context, taskID string) ([]Comment, error) {
	return c.TaskCommentsBefore(ctx, taskID, nil)
}

// TaskCommentsBefore returns the page of top-level comments older than before (newest first); a
// nil before means the newest page. ClickUp pages with the date and ID of the oldest comment seen.
func (c *Client) TaskCommentsBefore(ctx context.Context, taskID string, before *Comment) ([]Comment, error) {
	var q url.Values
	if before != nil {
		q = url.Values{}
		q.Set("start", strings.TrimSpace(before.Date))
		q.Set("start_id", before.ID)
	}
	var out struct {
		Comments []Comment `json:"comments"`
	}
	if err := c.get(ctx, "/task/"+url.PathEscape(taskID)+"/comment", q, &out); err != nil {
		return nil, err
	}
	return out.Comments, nil
}

// Replies returns the threaded replies of a comment.
func (c *Client) Replies(ctx context.Context, commentID string) ([]Comment, error) {
	var out struct {
		Comments []Comment `json:"comments"`
	}
	if err := c.get(ctx, "/comment/"+url.PathEscape(commentID)+"/reply", nil, &out); err != nil {
		return nil, err
	}
	return out.Comments, nil
}

func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	next := c.last.Add(c.gap)
	now := time.Now()
	if next.Before(now) {
		next = now
	}
	c.last = next
	c.mu.Unlock()

	if d := time.Until(next); d > 0 {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	if c.token == "" {
		return errors.New("falta el token de ClickUp en Orbit > Plugins > ClickUp")
	}
	if err := c.wait(ctx); err != nil {
		return err
	}

	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("error al crear la petición a ClickUp: %w", err)
	}
	// Personal tokens (pk_...) go as-is, without the "Bearer" prefix.
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("error de red al consultar ClickUp (%s): %w", path, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return ErrUnauthorized
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, path)
	case resp.StatusCode == http.StatusTooManyRequests:
		rl := &RateLimitError{}
		if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil && reset > 0 {
			rl.RetryAt = time.Unix(reset, 0)
		}
		return rl
	case resp.StatusCode >= 400:
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("ClickUp respondió HTTP %d en %s: %s", resp.StatusCode, path, strings.TrimSpace(string(b)))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("respuesta inválida de ClickUp en %s: %w", path, err)
	}
	return nil
}
