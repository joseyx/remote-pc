package clickup

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

// A note that cannot be written must not lose the mention: it is retried on the next poll and only
// counts as new once it is actually in the note.
func TestPollRetriesFailedNoteWrites(t *testing.T) {
	f := newFake()
	f.mine = []map[string]any{task("T1", "Mi ticket", meID)}
	f.comments["T1"] = []Comment{comment("c1", peerID, time.Now().Add(-time.Hour), "listo", 0)}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	notes := &fakeNotes{fail: 1}
	notified := 0
	p, store := newTestPoller(t, srv.URL, notes, &notified)

	res, err := p.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.New != 0 || len(res.Errors) != 1 || notified != 0 {
		t.Fatalf("failed write: res=%+v notified=%d", res, notified)
	}
	if got := store.snapshot().PendingNotes; len(got) != 1 || got[0].CommentID != "c1" {
		t.Fatalf("mention should be queued, got %+v", got)
	}

	res, err = p.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.New != 1 || len(notes.calls) != 1 || notes.calls[0].marker != "clickup:c1" || notified != 1 {
		t.Fatalf("retry: res=%+v calls=%+v notified=%d", res, notes.calls, notified)
	}
	if got := store.snapshot().PendingNotes; len(got) != 0 {
		t.Errorf("queue should be empty after the retry, got %+v", got)
	}
	if m := store.Mentions(MentionFilter{}); len(m) != 1 || m[0].NoteDate != "2026-11-26" {
		t.Errorf("stored mention after retry = %+v", m)
	}
}

// Tasks beyond the per-poll budget are not dropped: they keep their window and go first next time.
func TestPollBudgetKeepsCutTasks(t *testing.T) {
	const total = maxTasksPerPoll + 5
	f := newFake()
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("T%03d", i)
		f.mine = append(f.mine, task(id, "Ticket "+id, meID))
		f.comments[id] = []Comment{comment("c"+id, peerID, time.Now().Add(-time.Hour), "nuevo", 0)}
	}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	notes := &fakeNotes{}
	notified := 0
	p, store := newTestPoller(t, srv.URL, notes, &notified)

	res1, err := p.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res1.Checked != maxTasksPerPoll || res1.New != maxTasksPerPoll || !res1.Truncated {
		t.Fatalf("first poll = %+v", res1)
	}
	if got := len(store.snapshot().NextSince); got < total-maxTasksPerPoll {
		t.Fatalf("cut tasks should keep a cursor, NextSince has %d entries", got)
	}

	res2, err := p.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res2.New != total-maxTasksPerPoll {
		t.Fatalf("second poll should pick up the %d cut tasks, got %+v", total-maxTasksPerPoll, res2)
	}
	if len(notes.calls) != total {
		t.Errorf("expected %d to-dos in total, got %d", total, len(notes.calls))
	}
}

// More than one page of new comments: the older pages are read too.
func TestPollPaginatesLongTasks(t *testing.T) {
	f := newFake()
	f.mine = []map[string]any{task("L1", "Ticket largo", meID)}
	now := time.Now()
	for i := 29; i >= 0; i-- { // newest first, like ClickUp
		f.comments["L1"] = append(f.comments["L1"], comment(fmt.Sprintf("c%02d", i), peerID, now.Add(-time.Duration(30-i)*time.Minute), "x", 0))
	}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	notes := &fakeNotes{}
	notified := 0
	p, _ := newTestPoller(t, srv.URL, notes, &notified)
	res, err := p.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.New != 30 || len(notes.calls) != 30 {
		t.Fatalf("expected all 30 comments, got %+v (%d notes)", res, len(notes.calls))
	}
	if f.calls["/task/L1/comment"] != 2 {
		t.Errorf("expected 2 comment pages, got %d", f.calls["/task/L1/comment"])
	}
}

// A thread read in an earlier poll is checked from its own last read, so a reply older than the
// global window (the thread was skipped meanwhile) is still found.
func TestReplySinceOverridesTheGlobalWindow(t *testing.T) {
	since := t0
	act := TaskActivity{
		Task:       Task{ID: testTID, Name: "Tarea"},
		Comments:   []Comment{comment("root", meID, t0.Add(-5*time.Hour), "MR", 2)},
		Replies:    map[string][]Comment{"root": {comment("r1", peerID, t0.Add(-2*time.Hour), "visto", 0)}},
		ReplySince: map[string]time.Time{"root": t0.Add(-3 * time.Hour)},
	}
	if got := Detect(meID, act, since).Mentions; len(got) != 1 || got[0].CommentID != "r1" {
		t.Fatalf("reply after the thread's own window was missed: %+v", got)
	}
}

// My comment in another thread of the task does not answer a mention.
func TestTopLevelMentionNotAnsweredByAnotherThread(t *testing.T) {
	act := TaskActivity{
		Task: Task{ID: testTID, Name: "Tarea"},
		Comments: []Comment{
			comment("mine", meID, t0, "otra cosa", 0),
			comment("m1", peerID, t0.Add(-30*time.Minute), "revisa", 0, meID),
		},
	}
	got := Detect(meID, act, t0.Add(-time.Hour)).Mentions
	if len(got) != 1 || got[0].Answered {
		t.Fatalf("mention must stay unanswered: %+v", got)
	}
}

// Stop cancels a manual poll stuck on the network instead of waiting for it.
func TestStopCancelsAManualPoll(t *testing.T) {
	blocked := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(blocked)
		<-r.Context().Done()
	}))
	defer srv.Close()

	store, err := OpenStore(filepath.Join(t.TempDir(), "s.json"))
	if err != nil {
		t.Fatal(err)
	}
	oldGap := requestGap
	requestGap = 0
	t.Cleanup(func() { requestGap = oldGap })
	p := NewPoller(Options{Store: store, Notes: &fakeNotes{}, Config: func() Config {
		return Config{Enabled: true, Token: "pk_test", BaseURL: srv.URL}
	}})

	errc := make(chan error, 1)
	go func() { _, err := p.Poll(context.Background()); errc <- err }()
	<-blocked
	p.Stop()
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("cancelled poll should report an error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Stop did not cancel the manual poll")
	}
}

// With a cursor, mentions come oldest-detected first so a client advancing to the last one never
// skips items beyond the limit.
func TestMentionsCursorOrder(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "s.json"))
	if err != nil {
		t.Fatal(err)
	}
	st := s.snapshot()
	base := time.Now()
	for i := 1; i <= 3; i++ {
		d := base.Add(time.Duration(i) * time.Minute)
		st.Mentions = append(st.Mentions, Mention{CommentID: fmt.Sprintf("m%d", i), DetectedAt: d, Date: d})
	}
	if err := s.replace(st, base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got := s.Mentions(MentionFilter{DetectedAfter: base, Limit: 2})
	if len(got) != 2 || got[0].CommentID != "m1" || got[1].CommentID != "m2" {
		t.Fatalf("cursor page = %+v, want m1, m2", got)
	}
	if all := s.Mentions(MentionFilter{}); all[0].CommentID != "m3" {
		t.Errorf("without a cursor the newest comes first, got %s", all[0].CommentID)
	}
}
