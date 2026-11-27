package clickup

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultInterval and MinInterval bound how often the daemon polls ClickUp.
	DefaultInterval = 20 * time.Minute
	MinInterval     = 5 * time.Minute

	checkEvery      = time.Minute      // how often the loop re-reads the plugin config
	firstCheckDelay = 30 * time.Second // let the daemon finish starting before the first poll
	pollTimeout     = 3 * time.Minute

	// pollOverlap re-scans a few minutes before the last successful poll so comments posted while it
	// ran are not missed; already-seen comments are skipped.
	pollOverlap = 5 * time.Minute
	// initialLookback is the window of the very first poll.
	initialLookback = 24 * time.Hour
	// assignedWindow limits the "my tickets" scan to tickets touched recently.
	assignedWindow = 14 * 24 * time.Hour

	maxUpdatedPages   = 5
	maxAssignedPages  = 3
	maxTasksPerPoll   = 80
	maxThreadsPerTask = 10
)

// ErrDisabled means the ClickUp plugin is off.
var ErrDisabled = errors.New("el plugin ClickUp no está habilitado: actívalo en Orbit > Plugins")

// Config is the plugin configuration the poller reads before every check.
type Config struct {
	Enabled  bool
	Token    string
	TeamID   string
	Interval time.Duration
	BaseURL  string // empty = public API; tests point it at a fake server
}

// NoteSink receives the to-dos; calendar.Manager implements it.
type NoteSink interface {
	AddDailyTodoUnique(date string, text string, marker string) (bool, error)
}

// Options wires a Poller.
type Options struct {
	Store  *Store
	Notes  NoteSink
	Config func() Config
	// Notify is called after a poll that found new unanswered items.
	Notify func()
	// NoteDate maps an instant to the daily note it belongs to (the daemon uses the effective
	// business day, so 00:00-05:00 still counts for the previous day).
	NoteDate func(time.Time) string
	Now      func() time.Time
}

// PollResult summarises one poll.
type PollResult struct {
	New       int      `json:"new"`       // unanswered items added to the note
	Answered  int      `json:"answered"`  // items found already answered (stored, not announced)
	Checked   int      `json:"checked"`   // tasks scanned
	Truncated bool     `json:"truncated"` // some tasks or pages were left for the next poll
	Errors    []string `json:"errors,omitempty"`
}

// Poller periodically brings ClickUp comments addressed to the user into Orbit.
type Poller struct {
	opts Options

	pollMu sync.Mutex // one poll at a time (ticker and manual "poll now")

	lifeMu  sync.Mutex
	started bool
	stop    chan struct{}
	done    chan struct{}
}

// NewPoller builds a poller; Start launches its loop.
func NewPoller(o Options) *Poller {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.NoteDate == nil {
		o.NoteDate = func(t time.Time) string { return t.Format("2006-01-02") }
	}
	if o.Notify == nil {
		o.Notify = func() {}
	}
	return &Poller{opts: o, stop: make(chan struct{}), done: make(chan struct{})}
}

// Start launches the background loop. It is a no-op if already started.
func (p *Poller) Start() {
	p.lifeMu.Lock()
	defer p.lifeMu.Unlock()
	if p.started {
		return
	}
	p.started = true
	go p.loop()
}

// Stop ends the loop, cancelling a poll in flight, and waits for it to exit.
func (p *Poller) Stop() {
	p.lifeMu.Lock()
	if !p.started {
		p.lifeMu.Unlock()
		return
	}
	p.started = false
	close(p.stop)
	p.lifeMu.Unlock()
	<-p.done
}

func (p *Poller) loop() {
	defer close(p.done)
	first := time.NewTimer(firstCheckDelay)
	defer first.Stop()
	tick := time.NewTicker(checkEvery)
	defer tick.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-first.C:
			p.pollIfDue()
		case <-tick.C:
			p.pollIfDue()
		}
	}
}

// pollIfDue polls when the plugin is on and the interval since the last attempt has elapsed.
func (p *Poller) pollIfDue() {
	cfg := p.opts.Config()
	if !cfg.Enabled || strings.TrimSpace(cfg.Token) == "" {
		return
	}
	interval := cfg.Interval
	if interval < MinInterval {
		interval = MinInterval
	}
	if last := p.opts.Store.Status().LastPollStart; !last.IsZero() && p.opts.Now().Sub(last) < interval {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), pollTimeout)
	defer cancel()
	go func() {
		select {
		case <-p.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	if res, err := p.Poll(ctx); err != nil {
		log.Printf("[clickup] la revisión falló: %v", err)
	} else if res.New > 0 || len(res.Errors) > 0 {
		log.Printf("[clickup] %d nueva(s), %d tarea(s) revisada(s), %d error(es)", res.New, res.Checked, len(res.Errors))
	}
}

// Poll runs one scan now, whatever the interval says.
func (p *Poller) Poll(ctx context.Context) (PollResult, error) {
	p.pollMu.Lock()
	defer p.pollMu.Unlock()

	cfg := p.opts.Config()
	if !cfg.Enabled {
		return PollResult{}, ErrDisabled
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return PollResult{}, errors.New("falta el token de ClickUp en Orbit > Plugins > ClickUp")
	}

	start := p.opts.Now()
	st := p.opts.Store.snapshot()
	st.Status.LastPollStart = start

	var fresh []Mention
	res, err := p.scan(ctx, cfg, &st, start, &fresh)
	// Deliver what was found even if the scan stopped half-way: those comments are already marked seen.
	p.deliver(&st, fresh, &res)

	st.Status.LastPollEnd = p.opts.Now()
	st.Status.LastChecked = res.Checked
	switch {
	case err != nil:
		st.Status.LastError = err.Error()
	case len(res.Errors) > 0:
		// Keep the previous anchor so the failed tasks are re-scanned next time.
		st.Status.LastError = fmt.Sprintf("%d error(es); el primero: %s", len(res.Errors), res.Errors[0])
	case res.Truncated:
		// Updated tasks are scanned first, so what got cut are my tickets / watched tasks, which are
		// re-scanned every poll anyway: advance the anchor so the window does not keep growing.
		st.Status.LastError = "quedaron tareas sin revisar; se completan en la siguiente vuelta"
		st.Status.LastSuccess = start
	default:
		st.Status.LastError = ""
		st.Status.LastSuccess = start
	}

	if saveErr := p.opts.Store.replace(st, p.opts.Now()); saveErr != nil {
		log.Printf("[clickup] no se pudo guardar el estado: %v", saveErr)
		if err == nil {
			err = saveErr
		}
	}
	if res.New > 0 {
		p.opts.Notify()
	}
	return res, err
}

type candidate struct {
	task     Task
	assigned bool
}

// deliver writes the new unanswered mentions to the daily note, oldest first so the note (which
// inserts each line at the top of its section) reads newest-first, and stores them.
func (p *Poller) deliver(st *storeState, fresh []Mention, res *PollResult) {
	if len(fresh) == 0 {
		return
	}
	sort.SliceStable(fresh, func(i, j int) bool { return fresh[i].Date.Before(fresh[j].Date) })
	date := p.opts.NoteDate(p.opts.Now())
	for _, m := range fresh {
		if _, err := p.opts.Notes.AddDailyTodoUnique(date, RenderTodo(m, date), "clickup:"+m.CommentID); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("nota del %s: %v", date, err))
		} else {
			m.NoteDate = date
		}
		res.New++
		st.Mentions = append([]Mention{m}, st.Mentions...)
	}
}

// scan does the ClickUp work of one poll and records what it finds in st. New unanswered mentions
// are appended to fresh for deliver.
func (p *Poller) scan(ctx context.Context, cfg Config, st *storeState, start time.Time, fresh *[]Mention) (PollResult, error) {
	var res PollResult
	client := NewClient(cfg.BaseURL, cfg.Token)

	me, err := client.Me(ctx)
	if err != nil {
		return res, err
	}
	if st.Status.MeID != 0 && st.Status.MeID != me.ID {
		// The token now belongs to someone else: what we knew does not apply.
		fresh := emptyState()
		fresh.Status = st.Status
		fresh.Status.LastSuccess = time.Time{}
		*st = fresh
	}
	st.Status.MeID, st.Status.MeName = me.ID, strings.TrimSpace(me.Username)

	teamID := strings.TrimSpace(cfg.TeamID)
	if teamID == "" {
		teams, err := client.Teams(ctx)
		if err != nil {
			return res, err
		}
		if len(teams) == 0 {
			return res, errors.New("el token de ClickUp no tiene acceso a ningún workspace")
		}
		teamID = teams[0].ID
	}
	st.Status.TeamID = teamID

	since := start.Add(-initialLookback)
	if !st.Status.LastSuccess.IsZero() {
		since = st.Status.LastSuccess.Add(-pollOverlap)
	}

	cands, truncated, err := p.candidates(ctx, client, teamID, me.ID, st, since, start)
	if err != nil {
		return res, err
	}
	res.Truncated = truncated

	for _, c := range cands {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if err := p.scanTask(ctx, client, me.ID, c, st, since, start, &res, fresh); err != nil {
			if isFatal(err) {
				return res, err
			}
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", c.task.ID, err))
		}
	}
	return res, nil
}

// candidates lists the tasks to scan: updated since the last poll, my recent tickets, and tasks I
// wrote in recently (their threads get replies without the task's update date moving).
func (p *Poller) candidates(ctx context.Context, client *Client, teamID string, me int64, st *storeState, since, start time.Time) ([]candidate, bool, error) {
	byID := map[string]int{}
	var list []candidate
	add := func(t Task, assigned bool) {
		if !validTaskID(t.ID) {
			return
		}
		if i, ok := byID[t.ID]; ok {
			list[i].assigned = list[i].assigned || assigned
			if list[i].task.Name == "" {
				list[i].task.Name = t.Name
			}
			return
		}
		byID[t.ID] = len(list)
		list = append(list, candidate{task: t, assigned: assigned})
	}

	updated, truncU, err := client.Tasks(ctx, teamID, TaskQuery{UpdatedAfter: since, IncludeClosed: true, MaxPages: maxUpdatedPages})
	if err != nil {
		return nil, false, err
	}
	for _, t := range updated {
		add(t, t.IsAssignedTo(me))
	}

	mine, truncA, err := client.Tasks(ctx, teamID, TaskQuery{UpdatedAfter: start.Add(-assignedWindow), AssigneeID: me, MaxPages: maxAssignedPages})
	if err != nil {
		return nil, false, err
	}
	for _, t := range mine {
		add(t, true)
	}

	for id, w := range st.Watched {
		add(Task{ID: id, Name: w.Name}, false)
	}

	truncated := truncU || truncA
	if len(list) > maxTasksPerPoll {
		list = list[:maxTasksPerPoll]
		truncated = true
	}
	return list, truncated, nil
}

type threadUpdate struct {
	id    string
	state ThreadState
}

// scanTask fetches one task's comments and the threads that changed, then records the results.
// State is only touched once the task was fully read, so a failure leaves it to the next poll.
func (p *Poller) scanTask(ctx context.Context, client *Client, me int64, c candidate, st *storeState, since, start time.Time, res *PollResult, fresh *[]Mention) error {
	comments, err := client.TaskComments(ctx, c.task.ID)
	if err != nil {
		return err
	}

	act := TaskActivity{Task: c.task, AssignedToMe: c.assigned, Comments: comments, Replies: map[string][]Comment{}}
	var updates []threadUpdate
	fetched := 0
	for _, root := range comments {
		if root.ReplyCount == 0 {
			continue
		}
		known, ok := st.Threads[root.ID]
		recent := root.Time().After(start.Add(-watchRetention))
		if !recent && !(ok && known.Participated) {
			continue
		}
		if ok && known.ReplyCount == root.ReplyCount {
			continue // no new replies since we last read this thread
		}
		if fetched >= maxThreadsPerTask {
			res.Truncated = true
			break
		}
		replies, err := client.Replies(ctx, root.ID)
		if err != nil {
			return err
		}
		fetched++
		act.Replies[root.ID] = replies
		updates = append(updates, threadUpdate{id: root.ID, state: ThreadState{
			TaskID:       c.task.ID,
			ReplyCount:   root.ReplyCount,
			Participated: known.Participated,
			CheckedAt:    start,
		}})
	}

	dr := Detect(me, act, since)

	participated := map[string]bool{}
	for _, id := range dr.ParticipatedThreads {
		participated[id] = true
	}
	for _, u := range updates {
		u.state.Participated = u.state.Participated || participated[u.id]
		st.Threads[u.id] = u.state
	}
	if !dr.MyLastActivity.IsZero() && start.Sub(dr.MyLastActivity) <= watchRetention {
		w := st.Watched[c.task.ID]
		if dr.MyLastActivity.After(w.LastSeen) {
			w.LastSeen = dr.MyLastActivity
		}
		if name := strings.TrimSpace(c.task.Name); name != "" {
			w.Name = name
		}
		st.Watched[c.task.ID] = w
	}

	// Millisecond precision: the dashboard tracks what it announced with a ms timestamp.
	now := time.UnixMilli(p.opts.Now().UnixMilli())
	for _, m := range dr.Mentions {
		if _, seen := st.Seen[m.CommentID]; seen {
			continue
		}
		m.DetectedAt = now
		st.Seen[m.CommentID] = now
		if m.Answered {
			res.Answered++
			st.Mentions = append([]Mention{m}, st.Mentions...)
			continue
		}
		*fresh = append(*fresh, m) // deliver() writes it to the note and stores it
	}
	res.Checked++
	return nil
}

// isFatal reports errors that make the rest of the poll pointless.
func isFatal(err error) bool {
	var rl *RateLimitError
	return errors.Is(err, ErrUnauthorized) || errors.As(err, &rl) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

var taskIDRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func validTaskID(id string) bool { return taskIDRegex.MatchString(id) }

// RenderTodo is the line for a mention in the daily note of noteDate (YYYY-MM-DD), without the
// "- [ ]" prefix and the marker. The time shows the date too when the comment is from another day.
// Everything that comes from ClickUp is escaped: the note is rendered as HTML in the dashboard.
func RenderTodo(m Mention, noteDate string) string {
	icon, verb := "💬", "te mencionó en"
	switch m.Reason {
	case ReasonThread:
		icon, verb = "↩️", "respondió en tu hilo de"
	case ReasonTicket:
		icon, verb = "📝", "comentó en tu ticket"
	}
	author := m.Author
	if author == "" {
		author = "Alguien"
	}
	task := m.TaskName
	if task == "" {
		task = m.TaskID
	}

	when := m.Date.Local()
	stamp := when.Format("15:04")
	if when.Format("2006-01-02") != noteDate {
		stamp = when.Format("02/01 15:04")
	}

	line := fmt.Sprintf("%s **%s** %s [%s](%s)", icon, EscapeMarkdown(author), verb, EscapeMarkdown(task), TaskURL(m.TaskID))
	if m.Text != "" {
		line += ": «" + EscapeMarkdown(m.Text) + "»"
	}
	return line + " · " + stamp
}

var markdownEscaper = strings.NewReplacer(
	// HTML first: the dashboard renders the note with marked straight into innerHTML.
	"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;",
	// Markdown syntax that could turn text into links, emphasis or code.
	`\`, `\\`, "`", "\\`", "*", `\*`, "_", `\_`, "[", `\[`, "]", `\]`, "(", `\(`, ")", `\)`,
	"~", `\~`, "|", `\|`, "!", `\!`, "#", `\#`,
	"\r", " ", "\n", " ",
)

// EscapeMarkdown makes untrusted text safe to embed inline in the daily note.
func EscapeMarkdown(s string) string {
	return markdownEscaper.Replace(s)
}
