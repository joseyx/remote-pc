package clickup

import (
	"context"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

// With the daily note off (the default) mentions only go to the inbox.
func TestInboxModeLeavesTheNoteAlone(t *testing.T) {
	now := time.Now()
	f := newFake()
	f.mine = []map[string]any{task("T1", "Mi ticket", meID)}
	f.comments["T1"] = []Comment{comment("c1", peerID, now.Add(-time.Hour), "listo para QA", 0)}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	notes := &fakeNotes{}
	notified := 0
	p, store := newTestPollerWith(t, srv.URL, notes, &notified, false)
	res, err := p.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.New != 1 || notified != 1 || len(notes.calls) != 0 {
		t.Fatalf("res=%+v notified=%d note calls=%d, want 1 new, 1 notify and no note write", res, notified, len(notes.calls))
	}
	if store.PendingCount() != 1 {
		t.Fatalf("pending = %d, want 1", store.PendingCount())
	}
	if m := store.Mentions(MentionFilter{})[0]; m.CommentID != "c1" || m.NoteDate != "" {
		t.Errorf("inbox item = %+v", m)
	}
}

// Replying in the thread of a pending mention resolves it, even in a task that is neither mine nor
// updated (a reply does not move the task's update date) and that I never wrote in before.
func TestReplyingInTheThreadResolvesTheMention(t *testing.T) {
	now := time.Now()
	f := newFake()
	f.tasks = []map[string]any{task("T2", "Ticket ajeno", peerID)}
	f.comments["T2"] = []Comment{comment("c1", peerID, now.Add(-time.Hour), "¿lo revisas?", 0, meID)}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	notified := 0
	p, store := newTestPollerWith(t, srv.URL, &fakeNotes{}, &notified, false)
	if _, err := p.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.PendingCount() != 1 {
		t.Fatalf("pending after the mention = %d, want 1", store.PendingCount())
	}

	f.mu.Lock()
	f.tasks = nil
	f.comments["T2"][0].ReplyCount = 1
	f.replies["c1"] = []Comment{comment("r1", meID, time.Now(), "ya quedó", 0)}
	f.mu.Unlock()

	res, err := p.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Resolved != 1 || store.PendingCount() != 0 || notified != 2 {
		t.Fatalf("res=%+v pending=%d notified=%d, want the mention resolved and the pages notified", res, store.PendingCount(), notified)
	}
	all := store.Mentions(MentionFilter{IncludeResolved: true})
	if len(all) != 1 || !all[0].Answered {
		t.Errorf("stored = %+v", all)
	}
}

// A manual poll with a lookback recovers a comment older than the regular window.
func TestLookbackRecoversOlderComments(t *testing.T) {
	now := time.Now()
	f := newFake()
	f.mine = []map[string]any{task("T1", "Mi ticket", meID)}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	p, store := newTestPollerWith(t, srv.URL, &fakeNotes{}, new(int), false)
	if _, err := p.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.comments["T1"] = []Comment{comment("old", peerID, now.Add(-30*time.Hour), "sigue sin extraer", 0)}
	f.mu.Unlock()

	if res, err := p.Poll(context.Background()); err != nil || res.New != 0 {
		t.Fatalf("regular poll = %+v, %v; the comment is older than its window", res, err)
	}
	res, err := p.PollWith(context.Background(), PollOptions{Lookback: 72 * time.Hour})
	if err != nil || res.New != 1 || store.PendingCount() != 1 {
		t.Fatalf("lookback poll = %+v, %v, pending %d; want the old comment", res, err, store.PendingCount())
	}
	if res, _ := p.PollWith(context.Background(), PollOptions{Lookback: 72 * time.Hour}); res.New != 0 {
		t.Errorf("a second lookback must not repeat it: %+v", res)
	}
}

// The very first poll covers a weekend.
func TestFirstPollLooksBackThreeDays(t *testing.T) {
	now := time.Now()
	f := newFake()
	f.mine = []map[string]any{task("T1", "Mi ticket", meID)}
	f.comments["T1"] = []Comment{
		comment("c2", peerID, now.Add(-60*time.Hour), "del viernes", 0),
		comment("c1", peerID, now.Add(-80*time.Hour), "demasiado viejo", 0),
	}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	p, store := newTestPollerWith(t, srv.URL, &fakeNotes{}, new(int), false)
	res, err := p.Poll(context.Background())
	if err != nil || res.New != 1 {
		t.Fatalf("first poll = %+v, %v; want only the 60 h old comment", res, err)
	}
	if got := store.Mentions(MentionFilter{})[0].CommentID; got != "c2" {
		t.Errorf("found %s, want c2", got)
	}
}

func seedMentions(t *testing.T, ms ...Mention) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s.json")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	st := s.snapshot()
	st.Mentions = append(st.Mentions, ms...)
	if err := s.replace(st, time.Now()); err != nil {
		t.Fatal(err)
	}
	return s, path
}

// Marking attended is saved, hides the item from the pending list and can be undone.
func TestSetDoneMarksAndReopens(t *testing.T) {
	now := time.Now()
	s, path := seedMentions(t, Mention{CommentID: "c1", Date: now, DetectedAt: now})

	if _, err := s.SetDone("nope", true, now); !errors.Is(err, ErrMentionNotFound) {
		t.Fatalf("unknown id: err = %v", err)
	}
	m, err := s.SetDone("c1", true, now)
	if err != nil || m.DoneAt == nil {
		t.Fatalf("SetDone = %+v, %v", m, err)
	}
	if s.PendingCount() != 0 || len(s.Mentions(MentionFilter{})) != 0 || len(s.Mentions(MentionFilter{IncludeResolved: true})) != 1 {
		t.Fatalf("an attended item must leave the pending list only")
	}
	reopened, err := OpenStore(path)
	if err != nil || reopened.PendingCount() != 0 {
		t.Fatalf("the mark was not saved: %v, pending %d", err, reopened.PendingCount())
	}
	if _, err := s.SetDone("c1", false, now); err != nil || s.PendingCount() != 1 {
		t.Fatalf("reopen: %v, pending %d", err, s.PendingCount())
	}
}

// A mark set while a poll runs survives the poll saving its (older) snapshot.
func TestSetDoneSurvivesAPollInFlight(t *testing.T) {
	now := time.Now()
	s, _ := seedMentions(t, Mention{CommentID: "c1", Date: now, DetectedAt: now})

	inFlight := s.snapshot() // the poll starts
	if _, err := s.SetDone("c1", true, now); err != nil {
		t.Fatal(err)
	}
	inFlight.Mentions[0].Answered = false // the poll only knows what it read
	if err := s.replace(inFlight, now); err != nil {
		t.Fatal(err)
	}
	if s.PendingCount() != 0 {
		t.Fatalf("the attended mark was lost when the poll saved")
	}
}

// Pending items are kept however old; resolved ones only for 30 days from their last change.
func TestPruneKeepsPendingAndAgesResolved(t *testing.T) {
	now := time.Now()
	old := now.Add(-40 * 24 * time.Hour)
	recent := now.Add(-time.Hour)
	s, _ := seedMentions(t,
		Mention{CommentID: "pending-old", Date: old, DetectedAt: old},
		Mention{CommentID: "answered-old", Date: old, DetectedAt: old, Answered: true},
		Mention{CommentID: "done-old", Date: old, DetectedAt: old, DoneAt: &old},
		Mention{CommentID: "done-recently", Date: old, DetectedAt: old, DoneAt: &recent},
	)
	got := map[string]bool{}
	for _, m := range s.Mentions(MentionFilter{IncludeResolved: true}) {
		got[m.CommentID] = true
	}
	if !got["pending-old"] || !got["done-recently"] || got["answered-old"] || got["done-old"] || len(got) != 2 {
		t.Errorf("kept %v, want pending-old and done-recently", got)
	}
}
