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

// More updated tasks than the listing pages hold: the next poll continues from the last task listed
// (no page is re-read, none is skipped, the first window does not slide) and every task is reached.
func TestPollContinuesAfterACutListing(t *testing.T) {
	const total = maxUpdatedPages*fakeTaskPage + 100
	f := newFake()
	f.pagedTasks = true
	base := time.Now().Add(-20 * time.Hour)
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("U%03d", i)
		at := base.Add(time.Duration(i) * time.Second)
		tk := task(id, "Ticket "+id, peerID)
		tk["date_updated"] = ms(at)
		f.tasks = append(f.tasks, tk)
		f.comments[id] = []Comment{comment("c"+id, peerID, at, "revisa", 0, meID)}
	}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	notes := &fakeNotes{}
	notified := 0
	p, store := newTestPoller(t, srv.URL, notes, &notified)

	if _, err := p.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	st := store.Status()
	wantResume := time.UnixMilli(base.Add(time.Duration(maxUpdatedPages*fakeTaskPage-1) * time.Second).UnixMilli()).Add(-time.Millisecond)
	if !st.NextWindow.Equal(wantResume) || !st.LastSuccess.IsZero() {
		t.Fatalf("after a cut listing: NextWindow=%v (want %v), LastSuccess=%v", st.NextWindow, wantResume, st.LastSuccess)
	}

	for i := 0; i < 15 && len(notes.calls) < total; i++ {
		if _, err := p.Poll(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(notes.calls) != total {
		t.Fatalf("reached %d of %d tasks", len(notes.calls), total)
	}
	seen := map[string]bool{}
	for _, c := range notes.calls {
		if seen[c.marker] {
			t.Fatalf("duplicate to-do %s", c.marker)
		}
		seen[c.marker] = true
	}
	if st := store.Status(); !st.NextWindow.IsZero() || st.LastSuccess.IsZero() {
		t.Errorf("once everything is listed the window should be normal again: %+v", st)
	}
}

// After a cut listing, a task listed later is still checked from the original window: a mention
// older than the cut point, on a task updated after it, is not dropped.
func TestPollKeepsTheDetectionWindowAfterACut(t *testing.T) {
	const filler = maxUpdatedPages * fakeTaskPage
	f := newFake()
	f.pagedTasks = true
	base := time.Now().Add(-20 * time.Hour)
	for i := 0; i < filler; i++ {
		id := fmt.Sprintf("F%03d", i)
		tk := task(id, "Relleno "+id, peerID)
		tk["date_updated"] = ms(base.Add(time.Duration(i) * time.Second))
		f.tasks = append(f.tasks, tk)
	}
	// X: mentioned before the cut point (filler-1 s), updated after it.
	x := task("X", "Ticket ajeno", peerID)
	x["date_updated"] = ms(base.Add(time.Duration(filler+100) * time.Second))
	f.tasks = append(f.tasks, x)
	f.comments["X"] = []Comment{comment("cx", peerID, base.Add(10*time.Second), "revisa", 0, meID)}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	notes := &fakeNotes{}
	notified := 0
	p, store := newTestPoller(t, srv.URL, notes, &notified)
	for i := 0; i < 12 && len(notes.calls) == 0; i++ {
		if _, err := p.Poll(context.Background()); err != nil {
			t.Fatal(err)
		}
		if i == 0 && store.Status().WindowStart.IsZero() {
			t.Fatal("a cut listing must keep the detection window")
		}
	}
	if len(notes.calls) != 1 || notes.calls[0].marker != "clickup:cx" {
		t.Fatalf("mention older than the cut point was dropped: %+v", notes.calls)
	}
}

// More than a cut's worth of tasks share one update date: the listing pages through them instead
// of jumping past the instant, and every task is reached.
func TestPollPagesThroughTasksWithTheSameDate(t *testing.T) {
	const total = maxUpdatedPages*fakeTaskPage + 150
	f := newFake()
	f.pagedTasks = true
	at := time.Now().Add(-10 * time.Hour)
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("S%03d", i)
		tk := task(id, "Masivo "+id, peerID)
		tk["date_updated"] = ms(at)
		f.tasks = append(f.tasks, tk)
		f.comments[id] = []Comment{comment("c"+id, peerID, at, "revisa", 0, meID)}
	}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	notes := &fakeNotes{}
	notified := 0
	p, store := newTestPoller(t, srv.URL, notes, &notified)
	// Poll 1 can still move the window up to just before the shared instant; poll 2 lists only tasks
	// of that instant, cannot move it further and must continue by page.
	for i := 0; i < 2; i++ {
		if _, err := p.Poll(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if st := store.Status(); st.NextPage != maxUpdatedPages || !st.NextWindow.Equal(time.UnixMilli(at.UnixMilli()).Add(-time.Millisecond)) {
		t.Fatalf("same-instant cut should continue by page, got NextPage=%d NextWindow=%v", st.NextPage, st.NextWindow)
	}
	for i := 0; i < 15 && len(notes.calls) < total; i++ {
		if _, err := p.Poll(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(notes.calls) != total {
		t.Fatalf("reached %d of %d tasks sharing one update date", len(notes.calls), total)
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

// New activity is read past the 4 pages used to look for old threads: 130 new comments, 6 pages.
func TestPollReadsEveryPageOfNewComments(t *testing.T) {
	const n = 130
	f := newFake()
	f.mine = []map[string]any{task("L2", "Ticket muy activo", meID)}
	now := time.Now()
	for i := n - 1; i >= 0; i-- { // newest first
		f.comments["L2"] = append(f.comments["L2"], comment(fmt.Sprintf("c%03d", i), peerID, now.Add(-time.Duration(n-i)*time.Minute), "x", 0))
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
	if res.New != n || res.Truncated {
		t.Fatalf("expected %d comments and a complete scan, got %+v", n, res)
	}
	if got := f.calls["/task/L2/comment"]; got != 6 {
		t.Errorf("expected 6 comment pages, got %d", got)
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

// A thread of mine whose root fell beyond the pages read in a long task keeps being checked.
func TestPollChecksMyThreadBeyondThePagesRead(t *testing.T) {
	now := time.Now()
	f := newFake()
	f.tasks = []map[string]any{task("W1", "Ticket ajeno largo", peerID)}
	f.comments["W1"] = []Comment{comment("root", meID, now.Add(-3*time.Hour), "MR con el fix", 1)}
	f.replies["root"] = []Comment{comment("r1", peerID, now.Add(-2*time.Hour), "visto", 0)}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	notes := &fakeNotes{}
	notified := 0
	p, _ := newTestPoller(t, srv.URL, notes, &notified)
	if _, err := p.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}

	// The task is no longer listed as updated, 110 older comments push my root past page 4, and a
	// new reply arrives in my thread.
	f.mu.Lock()
	f.tasks = nil
	var filler []Comment
	for i := 0; i < 110; i++ {
		filler = append(filler, comment(fmt.Sprintf("f%03d", i), peerID, now.Add(-150*time.Minute-time.Duration(i)*time.Second), "relleno", 0))
	}
	root := f.comments["W1"][0]
	root.ReplyCount = 2
	f.comments["W1"] = append(filler, root)
	f.replies["root"] = append(f.replies["root"], comment("r2", peerID, time.Now(), "mezclado", 0))
	f.mu.Unlock()

	res, err := p.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.New != 1 || notes.calls[len(notes.calls)-1].marker != "clickup:r2" {
		t.Fatalf("reply in my old thread was missed: %+v %+v", res, notes.calls)
	}
}

// A comment of mine with no replies yet is remembered as my thread: its first reply is found even
// after newer comments push it past the pages read.
func TestPollFindsTheFirstReplyToMyBuriedComment(t *testing.T) {
	now := time.Now()
	f := newFake()
	f.tasks = []map[string]any{task("W1", "Ticket ajeno largo", peerID)}
	f.comments["W1"] = []Comment{comment("root", meID, now.Add(-3*time.Hour), "MR con el fix", 0)}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	notes := &fakeNotes{}
	notified := 0
	p, _ := newTestPoller(t, srv.URL, notes, &notified)
	if _, err := p.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}

	f.mu.Lock()
	f.tasks = nil
	var filler []Comment
	for i := 0; i < 110; i++ {
		filler = append(filler, comment(fmt.Sprintf("f%03d", i), peerID, now.Add(-150*time.Minute-time.Duration(i)*time.Second), "relleno", 0))
	}
	root := f.comments["W1"][0]
	root.ReplyCount = 1
	f.comments["W1"] = append(filler, root)
	f.replies["root"] = []Comment{comment("r1", peerID, time.Now(), "¿ya está en develop?", 0)}
	f.mu.Unlock()

	res, err := p.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.New != 1 || notes.calls[len(notes.calls)-1].marker != "clickup:r1" {
		t.Fatalf("first reply to my buried comment was missed: %+v %+v", res, notes.calls)
	}
}

// Pending to-dos are not dropped by count: all of them survive a reopen of the store.
func TestPendingNotesAreNotCapped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	st := s.snapshot()
	now := time.Now()
	for i := 0; i < 150; i++ {
		// Half of them older than the 30-day history: still undelivered, so still kept.
		detected := now
		if i%2 == 0 {
			detected = now.Add(-40 * 24 * time.Hour)
		}
		st.PendingNotes = append(st.PendingNotes, Mention{CommentID: fmt.Sprintf("p%03d", i), DetectedAt: detected})
	}
	if err := s.replace(st, now); err != nil {
		t.Fatal(err)
	}
	s2, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(s2.snapshot().PendingNotes); got != 150 {
		t.Fatalf("pending after reopen = %d, want 150", got)
	}
}

// Mentions detected in the same instant are told apart by comment ID, so a page limit that splits
// them does not hide the rest.
func TestMentionsCursorBreaksTiesByCommentID(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "s.json"))
	if err != nil {
		t.Fatal(err)
	}
	st := s.snapshot()
	at := time.UnixMilli(time.Now().UnixMilli())
	for _, id := range []string{"c", "a", "b"} {
		st.Mentions = append(st.Mentions, Mention{CommentID: id, DetectedAt: at, Date: at})
	}
	if err := s.replace(st, at); err != nil {
		t.Fatal(err)
	}
	page1 := s.Mentions(MentionFilter{DetectedAfter: at.Add(-time.Second), Limit: 2})
	if len(page1) != 2 || page1[0].CommentID != "a" || page1[1].CommentID != "b" {
		t.Fatalf("page 1 = %+v", page1)
	}
	page2 := s.Mentions(MentionFilter{DetectedAfter: at, AfterID: "b", Limit: 2})
	if len(page2) != 1 || page2[0].CommentID != "c" {
		t.Fatalf("page 2 = %+v, want only c", page2)
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
