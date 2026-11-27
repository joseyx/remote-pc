package clickup

import (
	"encoding/json"
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
	// mentionRetention and maxMentions bound the history served to the UI and MCP.
	mentionRetention = 30 * 24 * time.Hour
	maxMentions      = 300
	maxPendingNotes  = 100
)

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
	LastError     string    `json:"lastError,omitempty"`
	LastChecked   int       `json:"lastChecked"` // tasks scanned in the last poll
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
	DetectedAfter   time.Time // only mentions detected after this instant
	Since           time.Time // only comments written after this instant
	IncludeAnswered bool
	Limit           int
}

// Mentions returns stored mentions, newest comment first. With DetectedAfter (a cursor of what a
// client already announced) they come oldest-detected first instead, so a client that advances its
// cursor to the last item returned never skips items beyond Limit.
func (s *Store) Mentions(f MentionFilter) []Mention {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Mention, 0)
	for _, m := range s.st.Mentions {
		if !f.IncludeAnswered && m.Answered {
			continue
		}
		if !f.DetectedAfter.IsZero() && !m.DetectedAt.After(f.DetectedAfter) {
			continue
		}
		if !f.Since.IsZero() && !m.Date.After(f.Since) {
			continue
		}
		out = append(out, m)
	}
	if !f.DetectedAfter.IsZero() {
		sort.SliceStable(out, func(i, j int) bool {
			if !out[i].DetectedAt.Equal(out[j].DetectedAt) {
				return out[i].DetectedAt.Before(out[j].DetectedAt)
			}
			return out[i].Date.Before(out[j].Date)
		})
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out
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

// replace prunes st, makes it the current state and writes it to disk atomically.
func (s *Store) replace(st storeState, now time.Time) error {
	st = prune(normalize(st), now)
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := writeFileAtomic(s.path, data); err != nil {
		return err
	}
	s.st = st
	return nil
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
	if len(st.PendingNotes) > maxPendingNotes {
		st.PendingNotes = st.PendingNotes[len(st.PendingNotes)-maxPendingNotes:]
	}
	sort.SliceStable(st.Mentions, func(i, j int) bool { return st.Mentions[i].Date.After(st.Mentions[j].Date) })
	kept := st.Mentions[:0]
	for _, m := range st.Mentions {
		if now.Sub(m.DetectedAt) > mentionRetention {
			continue
		}
		kept = append(kept, m)
		if len(kept) >= maxMentions {
			break
		}
	}
	st.Mentions = kept
	return st
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
