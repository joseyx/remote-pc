package clickup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	storeVersion = 1

	// watchRetention is how long a task stays watched after the user last wrote in it, and how long
	// a thread's reply count is remembered.
	watchRetention = 14 * 24 * time.Hour
	// seenRetention keeps processed comment IDs long enough to never re-announce one.
	seenRetention = 45 * 24 * time.Hour
	// mentionRetention and maxMentions bound the history of resolved mentions (answered or marked
	// attended) served to the UI and MCP. Pending ones stay until resolved, up to maxPending.
	mentionRetention = 30 * 24 * time.Hour
	maxMentions      = 300
	maxPending       = 1000
)

// ErrMentionNotFound is returned for a mention the store does not have (never detected or pruned).
var ErrMentionNotFound = errors.New("esa mención no está en la bandeja de ClickUp")

// WatchedTask is a task the user wrote in recently; it is re-scanned every poll because replies in
// its threads do not change the task's update date.
type WatchedTask struct {
	Name     string    `json:"name"`
	LastSeen time.Time `json:"lastSeen"`
}

// TaskCursor says from when a task's comments must be read on its next scan.
type TaskCursor struct {
	Since time.Time `json:"since"`
	Name  string    `json:"name,omitempty"` // kept so a task resumed from the backlog keeps its title
}

// ThreadState caches what the poller knows about a comment thread.
type ThreadState struct {
	TaskID       string    `json:"taskId"`
	ReplyCount   int       `json:"replyCount"`
	Participated bool      `json:"participated"`
	CheckedAt    time.Time `json:"checkedAt"`
}

// Status is the poller health shown in the UI and returned by the API.
type Status struct {
	MeID          int64     `json:"meId,omitempty"`
	MeName        string    `json:"meName,omitempty"`
	TeamID        string    `json:"teamId,omitempty"`
	LastPollStart time.Time `json:"lastPollStart,omitempty"`
	LastPollEnd   time.Time `json:"lastPollEnd,omitempty"`
	LastSuccess   time.Time `json:"lastSuccess,omitempty"`
	// NextWindow is where the next poll's listing resumes when the updated-tasks listing was cut:
	// the update date of the last task listed (listing is oldest first), so it never re-reads the
	// same pages nor skips the ones it did not reach. Zero once a poll lists everything.
	NextWindow time.Time `json:"nextWindow,omitempty"`
	// NextPage is the listing page to resume at when more than a cut's worth of tasks share the
	// same update date (NextWindow cannot move past them).
	NextPage int `json:"nextPage,omitempty"`
	// WindowStart is the detection window kept while catching up after a cut listing: tasks listed
	// later are still checked from here, not from NextWindow, so their older comments are not dropped.
	WindowStart time.Time `json:"windowStart,omitempty"`
	LastError   string    `json:"lastError,omitempty"`
	LastChecked int       `json:"lastChecked"` // tasks scanned in the last poll
}

type storeState struct {
	Version  int                    `json:"version"`
	Status   Status                 `json:"status"`
	Watched  map[string]WatchedTask `json:"watched"`
	Threads  map[string]ThreadState `json:"threads"`
	Seen     map[string]time.Time   `json:"seen"`
	Mentions []Mention              `json:"mentions"` // newest first
	// NextSince is the per-task cursor: the instant from which a task's comments must be read next
	// time. Tasks left out of a poll (budget, error, unread pages or threads) keep an old cursor, so
	// they are scanned first and from where they were left, even after the global window moved on.
	NextSince map[string]TaskCursor `json:"nextSince"`
	// PendingNotes are mentions whose to-do could not be written; they are retried every poll.
	PendingNotes []Mention `json:"pendingNotes,omitempty"`
}

// Store persists the poller state in a JSON file. It is safe for concurrent use.
type Store struct {
	mu   sync.RWMutex
	path string
	st   storeState
}

// OpenStore loads path, or starts empty when the file does not exist yet.
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, st: emptyState()}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("no se pudo leer %s: %w", path, err)
	}
	var st storeState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("%s está corrupto: %w", path, err)
	}
	s.st = normalize(st)
	return s, nil
}

func emptyState() storeState {
	return storeState{
		Version:   storeVersion,
		Watched:   map[string]WatchedTask{},
		Threads:   map[string]ThreadState{},
		Seen:      map[string]time.Time{},
		NextSince: map[string]TaskCursor{},
	}
}

func normalize(st storeState) storeState {
	st.Version = storeVersion
	if st.Watched == nil {
		st.Watched = map[string]WatchedTask{}
	}
	if st.Threads == nil {
		st.Threads = map[string]ThreadState{}
	}
	if st.Seen == nil {
		st.Seen = map[string]time.Time{}
	}
	if st.NextSince == nil {
		st.NextSince = map[string]TaskCursor{}
	}
	return st
}

// Status returns a copy of the poller status.
func (s *Store) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.st.Status
}

// MentionFilter narrows Mentions.
type MentionFilter struct {
	DetectedAfter time.Time // cursor: only mentions after (DetectedAfter, AfterID)
	AfterID       string    // tie-breaker for mentions detected in the same instant
	Since         time.Time // only comments written after this instant
	// IncludeResolved also returns mentions already answered or marked attended; by default only
	// pending ones are returned.
	IncludeResolved bool
	Limit           int
}

// Mentions returns stored mentions, newest comment first. With a cursor (DetectedAfter + AfterID,
// what a client already announced) they come ordered by (DetectedAt, CommentID) instead, and only
// those strictly after the cursor: a client that moves its cursor to the last item returned never
// skips any, even when a batch of mentions detected in the same instant crosses Limit.
func (s *Store) Mentions(f MentionFilter) []Mention {
	s.mu.RLock()
	defer s.mu.RUnlock()
	useCursor := !f.DetectedAfter.IsZero()
	out := make([]Mention, 0)
	for _, m := range s.st.Mentions {
		if !f.IncludeResolved && !m.Pending() {
			continue
		}
		if useCursor && !mentionAfter(m, f.DetectedAfter, f.AfterID) {
			continue
		}
		if !f.Since.IsZero() && !m.Date.After(f.Since) {
			continue
		}
		out = append(out, m)
	}
	if useCursor {
		sort.SliceStable(out, func(i, j int) bool {
			if !out[i].DetectedAt.Equal(out[j].DetectedAt) {
				return out[i].DetectedAt.Before(out[j].DetectedAt)
			}
			return out[i].CommentID < out[j].CommentID
		})
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out
}

// mentionAfter orders mentions by (DetectedAt, CommentID) for cursor pagination.
func mentionAfter(m Mention, t time.Time, id string) bool {
	return m.DetectedAt.After(t) || (m.DetectedAt.Equal(t) && m.CommentID > id)
}

// PendingCount is how many mentions still wait for the user (the inbox badge).
func (s *Store) PendingCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, m := range s.st.Mentions {
		if m.Pending() {
			n++
		}
	}
	return n
}

// SetDone marks a mention attended (done) or pending again, and saves the store. A poll running at
// the same time keeps the change: replace carries the user's DoneAt over to the state it saves.
func (s *Store) SetDone(commentID string, done bool, now time.Time) (Mention, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.st.Mentions {
		if s.st.Mentions[i].CommentID == commentID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Mention{}, ErrMentionNotFound
	}

	next := s.st
	next.Mentions = append([]Mention(nil), s.st.Mentions...)
	m := next.Mentions[idx]
	switch {
	case done && m.DoneAt == nil:
		t := now
		m.DoneAt = &t
	case !done:
		m.DoneAt = nil
	}
	next.Mentions[idx] = m

	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return Mention{}, err
	}
	if err := writeFileAtomic(s.path, data); err != nil {
		return Mention{}, err
	}
	s.st = next
	return m, nil
}

// snapshot returns a deep copy the poller can work on without holding the lock.
func (s *Store) snapshot() storeState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := storeState{
		Version:      s.st.Version,
		Status:       s.st.Status,
		Watched:      make(map[string]WatchedTask, len(s.st.Watched)),
		Threads:      make(map[string]ThreadState, len(s.st.Threads)),
		Seen:         make(map[string]time.Time, len(s.st.Seen)),
		Mentions:     append([]Mention(nil), s.st.Mentions...),
		NextSince:    make(map[string]TaskCursor, len(s.st.NextSince)),
		PendingNotes: append([]Mention(nil), s.st.PendingNotes...),
	}
	for k, v := range s.st.NextSince {
		cp.NextSince[k] = v
	}
	for k, v := range s.st.Watched {
		cp.Watched[k] = v
	}
	for k, v := range s.st.Threads {
		cp.Threads[k] = v
	}
	for k, v := range s.st.Seen {
		cp.Seen[k] = v
	}
	return cp
}

// replace prunes st, makes it the current state and writes it to disk atomically. st comes from a
// snapshot taken before the poll, so the attended marks (DoneAt, owned by the user) are taken from
// the current state: one set while the poll ran is not lost.
func (s *Store) replace(st storeState, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st = normalize(st)
	st.Mentions = withUserMarks(st.Mentions, s.st.Mentions)
	st = prune(st, now)
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFileAtomic(s.path, data); err != nil {
		return err
	}
	s.st = st
	return nil
}

// withUserMarks returns mentions with each DoneAt taken from current (same comment ID).
func withUserMarks(mentions, current []Mention) []Mention {
	if len(current) == 0 {
		return mentions
	}
	marks := make(map[string]*time.Time, len(current))
	for _, m := range current {
		marks[m.CommentID] = m.DoneAt
	}
	out := make([]Mention, len(mentions))
	for i, m := range mentions {
		if d, ok := marks[m.CommentID]; ok {
			m.DoneAt = d
		}
		out[i] = m
	}
	return out
}

func prune(st storeState, now time.Time) storeState {
	for id, w := range st.Watched {
		if now.Sub(w.LastSeen) > watchRetention {
			delete(st.Watched, id)
		}
	}
	for id, t := range st.Threads {
		if now.Sub(t.CheckedAt) > watchRetention && !t.Participated {
			delete(st.Threads, id)
		} else if now.Sub(t.CheckedAt) > seenRetention {
			delete(st.Threads, id)
		}
	}
	for id, t := range st.Seen {
		if now.Sub(t) > seenRetention {
			delete(st.Seen, id)
		}
	}
	for id, c := range st.NextSince {
		// A task starved for longer than this is given up (it would otherwise replay weeks of comments).
		if now.Sub(c.Since) > watchRetention {
			delete(st.NextSince, id)
		}
	}
	// PendingNotes are never pruned: their comments are already marked seen and would not come back,
	// so they stay queued until the note accepts them (deliver removes them once written).
	st.Mentions = pruneMentions(st.Mentions, now)
	return st
}

// pruneMentions keeps every pending mention (up to maxPending, newest first) and the resolved ones
// from the last mentionRetention (counted from when they were detected or marked attended, the
// later), up to maxMentions. The result is ordered newest comment first.
func pruneMentions(mentions []Mention, now time.Time) []Mention {
	sorted := append([]Mention(nil), mentions...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Date.After(sorted[j].Date) })
	kept := make([]Mention, 0, len(sorted))
	pending, resolved := 0, 0
	for _, m := range sorted {
		if m.Pending() {
			if pending >= maxPending {
				continue
			}
			pending++
		} else {
			ref := m.DetectedAt
			if m.DoneAt != nil && m.DoneAt.After(ref) {
				ref = *m.DoneAt
			}
			if now.Sub(ref) > mentionRetention || resolved >= maxMentions {
				continue
			}
			resolved++
		}
		kept = append(kept, m)
	}
	return kept
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("no se pudo crear el directorio de %s: %w", path, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("no se pudo escribir %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("no se pudo reemplazar %s: %w", path, err)
	}
	return nil
}
