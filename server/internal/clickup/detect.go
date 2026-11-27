package clickup

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Reason says why a comment was brought to the user.
type Reason string

const (
	// ReasonMention: the comment tags the user or is assigned to them.
	ReasonMention Reason = "mencion"
	// ReasonThread: a reply in a thread the user started or replied in.
	ReasonThread Reason = "hilo"
	// ReasonTicket: a comment on a ticket assigned to the user.
	ReasonTicket Reason = "ticket"
)

// maxTextRunes caps the comment excerpt kept per mention.
const maxTextRunes = 220

// Mention is a comment addressed to the user.
type Mention struct {
	CommentID  string    `json:"commentId"`
	ThreadID   string    `json:"threadId,omitempty"` // root comment when it is a reply
	TaskID     string    `json:"taskId"`
	TaskName   string    `json:"taskName"`
	TaskURL    string    `json:"taskUrl"`
	AuthorID   int64     `json:"authorId"`
	Author     string    `json:"author"`
	Text       string    `json:"text"`
	Date       time.Time `json:"date"`
	Reason     Reason    `json:"reason"`
	Answered   bool      `json:"answered"` // the user replied after it in the same thread (top-level comment: in its own thread)
	DetectedAt time.Time `json:"detectedAt"`
	NoteDate   string    `json:"noteDate,omitempty"` // daily note that got the to-do
	// DoneAt is when the user marked it attended in the inbox (nil = not marked). Only the user sets
	// it: the poller never changes it.
	DoneAt *time.Time `json:"doneAt,omitempty"`
}

// Pending reports whether the mention still waits for the user: not answered in its thread and not
// marked attended.
func (m Mention) Pending() bool {
	return !m.Answered && m.DoneAt == nil
}

// threadKey is the thread a mention belongs to: its root, or itself for a top-level comment (whose
// answers are the replies in its own thread).
func (m Mention) threadKey() string {
	if m.ThreadID != "" {
		return m.ThreadID
	}
	return m.CommentID
}

// TaskActivity is what the poller fetched for one task.
type TaskActivity struct {
	Task         Task
	AssignedToMe bool
	Comments     []Comment            // top-level comments
	Replies      map[string][]Comment // root comment ID -> replies (only threads that were fetched)
	// ReplySince overrides, per thread, the window for its replies: a thread read last at an older
	// instant is checked from then, so replies are not lost when the global window has moved on.
	ReplySince map[string]time.Time
}

// DetectResult is the outcome of scanning one task.
type DetectResult struct {
	Mentions []Mention
	// Threads where the user wrote the root or a reply (among the fetched ones).
	ParticipatedThreads []string
	// MyLastActivity is when the user last wrote in this task (zero if never, among what was fetched).
	MyLastActivity time.Time
	// MyLastReply maps each fetched thread the user replied in to their latest reply, so mentions
	// stored earlier in that thread can be marked answered.
	MyLastReply map[string]time.Time
}

// TaskURL is the canonical link to a task. It is built from the ID instead of trusting the API field,
// because the link ends up inside the daily note.
func TaskURL(taskID string) string {
	return "https://app.clickup.com/t/" + taskID
}

// Detect finds the comments in act addressed to user me and written after since.
func Detect(me int64, act TaskActivity, since time.Time) DetectResult {
	var res DetectResult
	base := Mention{
		TaskID:   act.Task.ID,
		TaskName: strings.TrimSpace(act.Task.Name),
		TaskURL:  TaskURL(act.Task.ID),
	}

	for _, c := range act.Comments {
		if c.User.ID == me {
			if t := c.Time(); t.After(res.MyLastActivity) {
				res.MyLastActivity = t
			}
		}
	}

	for _, c := range act.Comments {
		replies := act.Replies[c.ID]

		participated := c.User.ID == me
		var myLastReply time.Time
		for _, r := range replies {
			if r.User.ID == me {
				participated = true
				if t := r.Time(); t.After(myLastReply) {
					myLastReply = t
				}
			}
		}
		if participated {
			res.ParticipatedThreads = append(res.ParticipatedThreads, c.ID)
		}
		if !myLastReply.IsZero() {
			if res.MyLastReply == nil {
				res.MyLastReply = map[string]time.Time{}
			}
			res.MyLastReply[c.ID] = myLastReply
		}
		if myLastReply.After(res.MyLastActivity) {
			res.MyLastActivity = myLastReply
		}

		if reason, ok := classify(me, c, act.AssignedToMe, false); ok && c.Time().After(since) {
			m := fill(base, c, reason)
			// Only a reply of the user in this same thread answers it: a comment elsewhere in the
			// task may be about something else, and a wrongly "answered" item is never announced.
			m.Answered = myLastReply.After(m.Date)
			res.Mentions = append(res.Mentions, m)
		}

		replySince := since
		if t, ok := act.ReplySince[c.ID]; ok && t.Before(replySince) {
			replySince = t
		}
		for _, r := range replies {
			if !r.Time().After(replySince) {
				continue
			}
			reason, ok := classify(me, r, act.AssignedToMe, participated)
			if !ok {
				continue
			}
			m := fill(base, r, reason)
			m.ThreadID = c.ID
			m.Answered = myLastReply.After(m.Date)
			res.Mentions = append(res.Mentions, m)
		}
	}
	return res
}

// classify returns why comment c concerns user me, if it does. inMyThread is true for replies in a
// thread the user took part in.
func classify(me int64, c Comment, assignedToMe, inMyThread bool) (Reason, bool) {
	// Skip the user's own comments and system authors (ClickBot and automations have no real ID).
	if c.User.ID == me || c.User.ID <= 0 {
		return "", false
	}
	if tagsUser(c, me) || (c.Assignee != nil && c.Assignee.ID == me) {
		return ReasonMention, true
	}
	if inMyThread {
		return ReasonThread, true
	}
	if assignedToMe {
		return ReasonTicket, true
	}
	return "", false
}

func tagsUser(c Comment, userID int64) bool {
	for _, p := range c.Parts {
		if p.Type == "tag" && p.User != nil && p.User.ID == userID {
			return true
		}
	}
	return false
}

func fill(base Mention, c Comment, reason Reason) Mention {
	m := base
	m.CommentID = c.ID
	m.AuthorID = c.User.ID
	m.Author = strings.Join(strings.Fields(c.User.Username), " ") // usernames can carry tabs
	m.Text = excerpt(c.Text)
	m.Date = c.Time()
	m.Reason = reason
	return m
}

// excerpt collapses whitespace and caps the comment text.
func excerpt(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= maxTextRunes {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:maxTextRunes])) + "…"
}
