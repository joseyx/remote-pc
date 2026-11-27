package clickup

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	meID    int64 = 100
	peerID  int64 = 200
	qaID    int64 = 300
	botID   int64 = -1
	testTID       = "86abc"
)

var t0 = time.Date(2026, 11, 26, 13, 0, 0, 0, time.Local)

func ms(t time.Time) string { return strconv.FormatInt(t.UnixMilli(), 10) }

func comment(id string, author int64, at time.Time, text string, replies int, tags ...int64) Comment {
	parts := []CommentPart{{Text: text}}
	for _, u := range tags {
		parts = append(parts, CommentPart{Type: "tag", Text: "@x", User: &User{ID: u}})
	}
	return Comment{ID: id, Parts: parts, Text: text, User: User{ID: author, Username: "U" + strconv.FormatInt(author, 10)}, Date: ms(at), ReplyCount: replies}
}

func reasons(ms []Mention) map[string]Reason {
	out := map[string]Reason{}
	for _, m := range ms {
		out[m.CommentID] = m.Reason
	}
	return out
}

func TestDetectRules(t *testing.T) {
	since := t0.Add(-time.Hour)
	cases := []struct {
		name     string
		assigned bool
		comments []Comment
		replies  map[string][]Comment
		want     map[string]Reason
	}{
		{
			name:     "tag in a top-level comment on someone else's ticket",
			comments: []Comment{comment("c1", peerID, t0, "revisa", 0, meID)},
			want:     map[string]Reason{"c1": ReasonMention},
		},
		{
			name:     "untagged comment on my ticket",
			assigned: true,
			comments: []Comment{comment("c1", peerID, t0, "listo", 0)},
			want:     map[string]Reason{"c1": ReasonTicket},
		},
		{
			name:     "untagged comment on someone else's ticket is ignored",
			comments: []Comment{comment("c1", peerID, t0, "listo", 0)},
			want:     map[string]Reason{},
		},
		{
			name:     "my own comments and bots are ignored",
			assigned: true,
			comments: []Comment{comment("c1", meID, t0, "yo", 0, meID), comment("c2", botID, t0, "bot", 0, meID)},
			want:     map[string]Reason{},
		},
		{
			name:     "comments before the window are ignored",
			comments: []Comment{comment("c1", peerID, since.Add(-time.Minute), "viejo", 0, meID)},
			want:     map[string]Reason{},
		},
		{
			name:     "reply in a thread I started",
			comments: []Comment{comment("root", meID, t0.Add(-2*time.Hour), "MR !1", 1)},
			replies:  map[string][]Comment{"root": {comment("r1", peerID, t0, "visto", 0)}},
			want:     map[string]Reason{"r1": ReasonThread},
		},
		{
			name:     "reply in a thread where I replied",
			comments: []Comment{comment("root", qaID, t0.Add(-3*time.Hour), "error", 2)},
			replies: map[string][]Comment{"root": {
				comment("r1", meID, t0.Add(-2*time.Hour), "lo veo", 0),
				comment("r2", peerID, t0, "gracias", 0),
			}},
			want: map[string]Reason{"r2": ReasonThread},
		},
		{
			name:     "tag in a reply of a thread I am not in",
			comments: []Comment{comment("root", qaID, t0.Add(-3*time.Hour), "error", 1)},
			replies:  map[string][]Comment{"root": {comment("r1", peerID, t0, "revisa esto", 0, meID)}},
			want:     map[string]Reason{"r1": ReasonMention},
		},
		{
			name:     "untagged reply in other people's thread on someone else's ticket",
			comments: []Comment{comment("root", qaID, t0.Add(-3*time.Hour), "error", 1)},
			replies:  map[string][]Comment{"root": {comment("r1", peerID, t0, "ya en prod", 0)}},
			want:     map[string]Reason{},
		},
		{
			name:     "assigned comment counts as a mention",
			comments: []Comment{func() Comment { c := comment("c1", peerID, t0, "tuyo", 0); c.Assignee = &User{ID: meID}; return c }()},
			want:     map[string]Reason{"c1": ReasonMention},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			act := TaskActivity{Task: Task{ID: testTID, Name: "Tarea"}, AssignedToMe: tc.assigned, Comments: tc.comments, Replies: tc.replies}
			got := reasons(Detect(meID, act, since).Mentions)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for id, r := range tc.want {
				if got[id] != r {
					t.Fatalf("comment %s: got %q, want %q (all: %v)", id, got[id], r, got)
				}
			}
		})
	}
}

func TestDetectAnsweredAndParticipation(t *testing.T) {
	since := t0.Add(-time.Hour)
	act := TaskActivity{
		Task: Task{ID: testTID, Name: "Tarea"},
		Comments: []Comment{
			comment("root", qaID, t0.Add(-30*time.Minute), "me arroja un error", 2, meID),
		},
		Replies: map[string][]Comment{"root": {
			comment("r1", peerID, t0.Add(-20*time.Minute), "revisa", 0, meID),
			comment("r2", meID, t0.Add(-10*time.Minute), "ya lo veo", 0),
		}},
	}
	res := Detect(meID, act, since)
	if len(res.Mentions) != 2 {
		t.Fatalf("expected 2 mentions, got %+v", res.Mentions)
	}
	for _, m := range res.Mentions {
		if !m.Answered {
			t.Errorf("%s should be answered by my later reply", m.CommentID)
		}
	}
	if len(res.ParticipatedThreads) != 1 || res.ParticipatedThreads[0] != "root" {
		t.Errorf("participation = %v, want [root]", res.ParticipatedThreads)
	}
	if !res.MyLastActivity.Equal(time.UnixMilli(t0.Add(-10 * time.Minute).UnixMilli())) {
		t.Errorf("MyLastActivity = %v", res.MyLastActivity)
	}
	for _, m := range res.Mentions {
		if m.TaskURL != "https://app.clickup.com/t/"+testTID {
			t.Errorf("TaskURL = %q", m.TaskURL)
		}
		if m.CommentID == "r1" && m.ThreadID != "root" {
			t.Errorf("reply should carry its thread id, got %q", m.ThreadID)
		}
	}
}

func TestExcerptCollapsesAndCaps(t *testing.T) {
	long := strings.Repeat("palabra ", 100)
	got := excerpt("  hola\n\n  mundo  ")
	if got != "hola mundo" {
		t.Errorf("excerpt collapse = %q", got)
	}
	if r := []rune(excerpt(long)); len(r) > maxTextRunes+1 {
		t.Errorf("excerpt not capped: %d runes", len(r))
	}
}

func TestRenderTodoEscapesUntrustedText(t *testing.T) {
	m := Mention{
		CommentID: "c1", TaskID: testTID, TaskName: "Fix <script>alert(1)</script>",
		Author: "Ana [x](javascript:alert(1))", Text: "mira **esto** <img src=x onerror=alert(1)> `rm`",
		Date: t0, Reason: ReasonMention,
	}
	line := RenderTodo(m, "2026-11-26")
	for _, bad := range []string{"<script>", "<img", "](javascript:", "**esto**", "`rm`"} {
		if strings.Contains(line, bad) {
			t.Errorf("rendered line still contains %q: %s", bad, line)
		}
	}
	if !strings.Contains(line, "](https://app.clickup.com/t/"+testTID+")") {
		t.Errorf("rendered line lacks the task link: %s", line)
	}
	if !strings.HasSuffix(line, "· 13:00") {
		t.Errorf("same-day mention should show only the time: %s", line)
	}
	if other := RenderTodo(m, "2026-11-28"); !strings.HasSuffix(other, "· 26/11 13:00") {
		t.Errorf("older mention should show the date: %s", other)
	}
}
