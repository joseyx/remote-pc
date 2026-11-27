package clickup

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClickUp serves the few API v2 endpoints the poller uses and counts calls per path.
type fakeClickUp struct {
	mu       sync.Mutex
	calls    map[string]int
	tasks    []map[string]any // all tasks (listing without assignee filter)
	mine     []map[string]any // tasks returned for assignees[]=me
	comments map[string][]Comment
	replies  map[string][]Comment
	status   int // forced status for every request (0 = normal)
	// pagedTasks makes the task listing behave like ClickUp: date_updated_gt filter, order by
	// update date (ascending with reverse=true), pages of 100 and last_page.
	pagedTasks bool
}

func newFake() *fakeClickUp {
	return &fakeClickUp{calls: map[string]int{}, comments: map[string][]Comment{}, replies: map[string][]Comment{}}
}

func (f *fakeClickUp) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.calls[r.URL.Path]++
		if r.Header.Get("Authorization") != "pk_test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if f.status != 0 {
			w.WriteHeader(f.status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		switch {
		case r.URL.Path == "/user":
			_ = enc.Encode(map[string]any{"user": map[string]any{"id": meID, "username": "José "}})
		case r.URL.Path == "/team":
			_ = enc.Encode(map[string]any{"teams": []map[string]any{{"id": "t1", "name": "WS"}}})
		case r.URL.Path == "/team/t1/task":
			list := f.tasks
			if r.URL.Query().Get("assignees[]") != "" {
				list = f.mine
			}
			if f.pagedTasks {
				_ = enc.Encode(pageTasks(list, r.URL.Query()))
				return
			}
			_ = enc.Encode(map[string]any{"tasks": list, "last_page": true})
		case strings.HasPrefix(r.URL.Path, "/task/") && strings.HasSuffix(r.URL.Path, "/comment"):
			// Like ClickUp: newest first, pages of 25, older pages via start/start_id of the oldest seen.
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/task/"), "/comment")
			list := f.comments[id]
			if sid := r.URL.Query().Get("start_id"); sid != "" {
				for i, c := range list {
					if c.ID == sid {
						list = list[i+1:]
						break
					}
				}
			}
			if len(list) > CommentPageSize {
				list = list[:CommentPageSize]
			}
			_ = enc.Encode(map[string]any{"comments": list})
		case strings.HasPrefix(r.URL.Path, "/comment/") && strings.HasSuffix(r.URL.Path, "/reply"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/comment/"), "/reply")
			_ = enc.Encode(map[string]any{"comments": f.replies[id]})
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

const fakeTaskPage = 100

func pageTasks(all []map[string]any, q url.Values) map[string]any {
	gt, _ := strconv.ParseInt(q.Get("date_updated_gt"), 10, 64)
	upd := func(t map[string]any) int64 { v, _ := strconv.ParseInt(t["date_updated"].(string), 10, 64); return v }
	var list []map[string]any
	for _, t := range all {
		if upd(t) > gt {
			list = append(list, t)
		}
	}
	asc := q.Get("reverse") == "true"
	sort.SliceStable(list, func(i, j int) bool {
		if asc {
			return upd(list[i]) < upd(list[j])
		}
		return upd(list[i]) > upd(list[j])
	})
	page, _ := strconv.Atoi(q.Get("page"))
	from, to := page*fakeTaskPage, (page+1)*fakeTaskPage
	if from > len(list) {
		from = len(list)
	}
	if to > len(list) {
		to = len(list)
	}
	return map[string]any{"tasks": list[from:to], "last_page": to >= len(list)}
}

func task(id, name string, assignees ...int64) map[string]any {
	as := []map[string]any{}
	for _, a := range assignees {
		as = append(as, map[string]any{"id": a})
	}
	return map[string]any{"id": id, "name": name, "date_updated": ms(time.Now()), "assignees": as}
}

type noteCall struct{ date, text, marker string }

type fakeNotes struct {
	mu    sync.Mutex
	calls []noteCall
	fail  int // number of upcoming writes that fail
}

func (n *fakeNotes) AddDailyTodoUnique(date, text, marker string) (bool, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.fail > 0 {
		n.fail--
		return false, errors.New("nota bloqueada")
	}
	n.calls = append(n.calls, noteCall{date, text, marker})
	return true, nil
}

func newTestPoller(t *testing.T, baseURL string, notes NoteSink, notified *int) (*Poller, *Store) {
	t.Helper()
	oldGap := requestGap
	requestGap = 0
	t.Cleanup(func() { requestGap = oldGap })

	store, err := OpenStore(filepath.Join(t.TempDir(), "clickup_mentions.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := NewPoller(Options{
		Store: store,
		Notes: notes,
		Config: func() Config {
			return Config{Enabled: true, Token: "pk_test", BaseURL: baseURL, Interval: DefaultInterval}
		},
		Notify:   func() { *notified++ },
		NoteDate: func(time.Time) string { return "2026-11-26" },
	})
	return p, store
}

func TestPollFindsMentionsOnceAndCachesThreads(t *testing.T) {
	now := time.Now()
	f := newFake()
	f.tasks = []map[string]any{
		task("T1", "Mi ticket", meID),
		task("T2", "Ticket de otra persona", qaID),
		task("T4", "Ruido", peerID),
	}
	f.mine = []map[string]any{task("T1", "Mi ticket", meID), task("T5", "Ya respondido", meID)}
	f.comments["T1"] = []Comment{comment("c1", peerID, now.Add(-time.Hour), "listo para QA", 0)}
	f.comments["T2"] = []Comment{comment("c3", qaID, now.Add(-2*time.Hour), "me arroja error", 1)}
	// r1 is older than c1 but its task is scanned later: the note must still get it first.
	f.replies["c3"] = []Comment{comment("r1", peerID, now.Add(-90*time.Minute), "revisa esto", 0, meID)}
	f.comments["T4"] = []Comment{comment("c5", peerID, now.Add(-time.Hour), "nada que ver", 0)}
	f.comments["T5"] = []Comment{comment("c6", peerID, now.Add(-time.Hour), "¿y esto?", 1)}
	f.replies["c6"] = []Comment{comment("r6", meID, now.Add(-30*time.Minute), "ya lo voy a revisar", 0)}

	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	notes := &fakeNotes{}
	notified := 0
	p, store := newTestPoller(t, srv.URL, notes, &notified)

	res, err := p.Poll(context.Background())
	if err != nil {
		t.Fatalf("first poll: %v", err)
	}
	if res.New != 2 || res.Answered != 1 {
		t.Fatalf("first poll result = %+v, want 2 new (c1 ticket, r1 mention) and 1 answered (c6)", res)
	}
	if notified != 1 {
		t.Errorf("notify called %d times, want 1", notified)
	}
	markers := map[string]bool{}
	for _, c := range notes.calls {
		markers[c.marker] = true
		if c.date != "2026-11-26" {
			t.Errorf("to-do went to %s", c.date)
		}
	}
	if !markers["clickup:c1"] || !markers["clickup:r1"] || len(notes.calls) != 2 {
		t.Errorf("note calls = %+v", notes.calls)
	}
	if len(notes.calls) == 2 && notes.calls[0].marker != "clickup:r1" {
		t.Errorf("to-dos must be written oldest first, got %s then %s", notes.calls[0].marker, notes.calls[1].marker)
	}

	all := store.Mentions(MentionFilter{IncludeAnswered: true})
	if len(all) != 3 {
		t.Fatalf("stored %d mentions, want 3", len(all))
	}
	if pending := store.Mentions(MentionFilter{}); len(pending) != 2 {
		t.Errorf("unanswered = %d, want 2", len(pending))
	}
	st := store.Status()
	if st.MeID != meID || st.TeamID != "t1" || st.LastError != "" || st.LastSuccess.IsZero() {
		t.Errorf("status after first poll = %+v", st)
	}

	repliesBefore := f.calls["/comment/c3/reply"]

	// Second poll: nothing new, and unchanged threads are not fetched again.
	res2, err := p.Poll(context.Background())
	if err != nil {
		t.Fatalf("second poll: %v", err)
	}
	if res2.New != 0 || res2.Answered != 0 || len(notes.calls) != 2 || notified != 1 {
		t.Errorf("second poll should be quiet: %+v, notes=%d, notified=%d", res2, len(notes.calls), notified)
	}
	if f.calls["/comment/c3/reply"] != repliesBefore {
		t.Errorf("thread c3 was re-fetched although its reply count did not change")
	}

	// A new reply in the thread (count changes) is picked up.
	f.mu.Lock()
	f.comments["T2"][0].ReplyCount = 2
	f.replies["c3"] = append(f.replies["c3"], comment("r2", peerID, time.Now(), "¿ya?", 0, meID))
	f.mu.Unlock()
	res3, err := p.Poll(context.Background())
	if err != nil {
		t.Fatalf("third poll: %v", err)
	}
	if res3.New != 1 || !strings.Contains(notes.calls[len(notes.calls)-1].marker, "r2") {
		t.Errorf("third poll = %+v, last note = %+v", res3, notes.calls[len(notes.calls)-1])
	}
}

func TestPollWatchesTasksIWroteIn(t *testing.T) {
	now := time.Now()
	f := newFake()
	// T3 is not mine and is listed as updated only on the first poll.
	f.tasks = []map[string]any{task("T3", "Ticket ajeno", peerID)}
	f.comments["T3"] = []Comment{comment("root", meID, now.Add(-2*time.Hour), "MR con el fix", 0)}

	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()
	notes := &fakeNotes{}
	notified := 0
	p, _ := newTestPoller(t, srv.URL, notes, &notified)

	if _, err := p.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Later a reply arrives in my thread; replies do not move the task's update date.
	f.mu.Lock()
	f.tasks = nil
	f.comments["T3"][0].ReplyCount = 1
	f.replies["root"] = []Comment{comment("r1", peerID, time.Now(), "mezclado", 0)}
	f.mu.Unlock()

	res, err := p.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.New != 1 || len(notes.calls) != 1 || notes.calls[0].marker != "clickup:r1" {
		t.Fatalf("reply in my thread on a watched task was missed: %+v %+v", res, notes.calls)
	}
	if !strings.Contains(notes.calls[0].text, "respondió en tu hilo de") {
		t.Errorf("unexpected wording: %s", notes.calls[0].text)
	}
}

func TestPollErrors(t *testing.T) {
	f := newFake()
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	notes := &fakeNotes{}
	notified := 0
	p, store := newTestPoller(t, srv.URL, notes, &notified)

	f.status = http.StatusTooManyRequests
	if _, err := p.Poll(context.Background()); err == nil {
		t.Fatal("expected a rate-limit error")
	}
	if st := store.Status(); st.LastError == "" || !st.LastSuccess.IsZero() {
		t.Errorf("failed poll must record the error and not advance the anchor: %+v", st)
	}

	disabled := NewPoller(Options{Store: store, Notes: notes, Config: func() Config { return Config{} }})
	if _, err := disabled.Poll(context.Background()); err != ErrDisabled {
		t.Errorf("disabled plugin: got %v, want ErrDisabled", err)
	}
}

func TestStopEndsTheLoop(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "s.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := NewPoller(Options{Store: store, Notes: &fakeNotes{}, Config: func() Config { return Config{} }})
	p.Start()
	done := make(chan struct{})
	go func() { p.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return")
	}
	p.Stop() // idempotent
}

func TestStorePersistsAcrossOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	st := s.snapshot()
	now := time.Now()
	st.Seen["c1"] = now
	st.Mentions = []Mention{{CommentID: "c1", DetectedAt: now, Date: now}}
	if err := s.replace(st, now); err != nil {
		t.Fatal(err)
	}
	s2, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.Mentions(MentionFilter{}); len(got) != 1 || got[0].CommentID != "c1" {
		t.Errorf("reloaded mentions = %+v", got)
	}
	if got := s2.Mentions(MentionFilter{DetectedAfter: now, AfterID: "c1"}); len(got) != 0 {
		t.Errorf("a cursor at the item itself should exclude it: %+v", got)
	}
}
