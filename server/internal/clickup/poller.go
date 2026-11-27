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
	// Pages of 25 top-level comments read per task and poll: while the pages still hold comments
	// inside the window there is new activity to read (up to 1,000 comments); only to find a thread
	// of mine among old comments, 4 pages (older roots are then checked directly).
	maxCommentPagesNew     = 40
	maxCommentPagesThreads = 4
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
	// Notify is called after a poll that added new unanswered items to the note.
	Notify func()
	// NoteDate maps an instant to the daily note it belongs to (the daemon uses the effective
	// business day, so 00:00-05:00 still counts for the previous day).
	NoteDate func(time.Time) string
	Now      func() time.Time
}

// PollResult summarises one poll.
type PollResult struct {
	New       int      `json:"new"`       // unanswered items written to the note
	Answered  int      `json:"answered"`  // items found already answered (stored, not announced)
	Checked   int      `json:"checked"`   // tasks scanned
	Truncated bool     `json:"truncated"` // some tasks, pages or threads were left for the next poll
	Errors    []string `json:"errors,omitempty"`
}

// Poller periodically brings ClickUp comments addressed to the user into Orbit.
type Poller struct {
	opts Options

	// sem lets one poll run at a time (ticker and manual "poll now"); waiting for it honours the
	// caller's context, so neither the loop nor Stop can hang behind a long manual poll.
	sem chan struct{}

	lifeMu  sync.Mutex
	started bool
	stopped bool
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
	return &Poller{opts: o, sem: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{})}
}

// Start launches the background loop. It is a no-op if already started or stopped.
func (p *Poller) Start() {
	p.lifeMu.Lock()
	defer p.lifeMu.Unlock()
	if p.started || p.stopped {
		return
	}
	p.started = true
	go p.loop()
}

// Stop cancels any poll in flight (including manual ones started through the API), ends the loop
// and waits for it to exit. It is idempotent.
func (p *Poller) Stop() {
	p.lifeMu.Lock()
	if p.stopped {
		p.lifeMu.Unlock()
		return
	}
	p.stopped = true
	close(p.stop)
	started := p.started
	p.lifeMu.Unlock()
	if started {
		<-p.done
	}
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
	if res, err := p.Poll(ctx); err != nil {
		log.Printf("[clickup] la revisión falló: %v", err)
	} else if res.New > 0 || len(res.Errors) > 0 {
		log.Printf("[clickup] %d nueva(s), %d tarea(s) revisada(s), %d error(es)", res.New, res.Checked, len(res.Errors))
	}
}

// Poll runs one scan now, whatever the interval says. Stop cancels it.
func (p *Poller) Poll(ctx context.Context) (PollResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-p.stop:
			cancel()
		case <-ctx.Done():
		}
	}()

	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		return PollResult{}, ctx.Err()
	}
	defer func() { <-p.sem }()

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
	res, resume, err := p.scan(ctx, cfg, &st, start, &fresh)
	// Deliver what was found even if the scan stopped half-way: those comments are already marked seen.
	p.deliver(&st, fresh, &res)

	st.Status.LastPollEnd = p.opts.Now()
	st.Status.LastChecked = res.Checked
	switch {
	case err != nil:
		st.Status.LastError = err.Error()
	case !resume.IsZero():
		// The updated-tasks listing was cut: everything up to resume was listed (oldest first), so
		// the next poll continues from there instead of re-reading the same pages.
		st.Status.NextWindow = resume
		st.Status.LastError = "hubo más tareas actualizadas de las que caben en una vuelta; la siguiente sigue desde " +
			resume.Local().Format("02/01 15:04:05")
	default:
		// Every task left out (budget, error, unread pages or threads) kept its own cursor, so the
		// global window can move on without losing them.
		st.Status.LastSuccess = start
		st.Status.NextWindow = time.Time{}
		st.Status.LastError = summarize(res)
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

func summarize(res PollResult) string {
	switch {
	case len(res.Errors) > 0:
		return fmt.Sprintf("%d error(es); el primero: %s", len(res.Errors), res.Errors[0])
	case res.Truncated:
		return "quedaron tareas o hilos para la siguiente vuelta"
	}
	return ""
}

// deliver writes new unanswered mentions, plus those whose to-do failed before, to the daily note.
// It goes oldest first so the note (which inserts each line at the top of its section) reads
// newest-first. A failed write keeps the mention queued for the next poll.
func (p *Poller) deliver(st *storeState, fresh []Mention, res *PollResult) {
	queue := append(append([]Mention(nil), st.PendingNotes...), fresh...)
	st.PendingNotes = nil
	if len(queue) == 0 {
		return
	}
	sort.SliceStable(queue, func(i, j int) bool { return queue[i].Date.Before(queue[j].Date) })
	date := p.opts.NoteDate(p.opts.Now())
	for _, m := range queue {
		if _, err := p.opts.Notes.AddDailyTodoUnique(date, RenderTodo(m, date), "clickup:"+m.CommentID); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("nota del %s: %v", date, err))
			st.PendingNotes = append(st.PendingNotes, m)
			upsertMention(st, m) // still listed in the API / MCP while the note is unavailable
			continue
		}
		m.NoteDate = date
		res.New++
		upsertMention(st, m)
	}
}

func upsertMention(st *storeState, m Mention) {
	for i := range st.Mentions {
		if st.Mentions[i].CommentID == m.CommentID {
			st.Mentions[i] = m
			return
		}
	}
	st.Mentions = append([]Mention{m}, st.Mentions...)
}

type candidate struct {
	task     Task
	assigned bool
	watched  bool
	rank     int       // 0 updated, 1 my ticket, 2 watched, 3 backlog: tie-breaker after since
	since    time.Time // window start for this task
}

// scan does the ClickUp work of one poll and records what it finds in st. New unanswered mentions
// are appended to fresh for deliver. resume is non-zero when the updated-tasks listing was cut: it
// is where the next poll's window must start.
func (p *Poller) scan(ctx context.Context, cfg Config, st *storeState, start time.Time, fresh *[]Mention) (res PollResult, resume time.Time, err error) {
	client := NewClient(cfg.BaseURL, cfg.Token)

	me, err := client.Me(ctx)
	if err != nil {
		return res, resume, err
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
			return res, resume, err
		}
		if len(teams) == 0 {
			return res, resume, errors.New("el token de ClickUp no tiene acceso a ningún workspace")
		}
		teamID = teams[0].ID
	}
	st.Status.TeamID = teamID

	since := start.Add(-initialLookback)
	if !st.Status.LastSuccess.IsZero() {
		since = st.Status.LastSuccess.Add(-pollOverlap)
	}
	if st.Status.NextWindow.After(since) {
		since = st.Status.NextWindow // a previous listing was cut: continue where it stopped
	}

	cands, resume, partial, err := p.candidates(ctx, client, teamID, me.ID, st, since, start)
	if err != nil {
		return res, time.Time{}, err
	}
	res.Truncated = partial
	if len(cands) > maxTasksPerPoll {
		// The least recently scanned go first (see candidates); the rest keep a cursor and lead next time.
		for _, c := range cands[maxTasksPerPoll:] {
			keepCursor(st, c)
		}
		cands = cands[:maxTasksPerPoll]
		res.Truncated = true
	}

	for i, c := range cands {
		if err := ctx.Err(); err != nil {
			for _, rest := range cands[i:] {
				keepCursor(st, rest)
			}
			return res, resume, err
		}
		complete, err := p.scanTask(ctx, client, me.ID, c, st, start, &res, fresh)
		switch {
		case errors.Is(err, ErrNotFound):
			// Deleted task or lost access: stop following it.
			delete(st.Watched, c.task.ID)
			delete(st.NextSince, c.task.ID)
		case err != nil && isFatal(err):
			for _, rest := range cands[i:] {
				keepCursor(st, rest)
			}
			return res, resume, err
		case err != nil:
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", c.task.ID, err))
			keepCursor(st, c)
		case !complete:
			res.Truncated = true
			keepCursor(st, c)
		case c.assigned || c.watched:
			st.NextSince[c.task.ID] = TaskCursor{Since: start.Add(-pollOverlap), Name: c.task.Name}
		default:
			delete(st.NextSince, c.task.ID) // a later update will list it again
		}
	}
	return res, resume, nil
}

// keepCursor makes sure a task that was not (fully) scanned is resumed from its own window.
func keepCursor(st *storeState, c candidate) {
	cur, ok := st.NextSince[c.task.ID]
	if !ok || c.since.Before(cur.Since) {
		cur.Since = c.since
	}
	if c.task.Name != "" {
		cur.Name = c.task.Name
	}
	st.NextSince[c.task.ID] = cur
}

// candidates lists the tasks to scan: updated since the last poll, my recent tickets, tasks I wrote
// in recently (their threads get replies without the task's update date moving) and the backlog of
// tasks left out before. They are ordered by how far behind they are, so nobody starves.
func (p *Poller) candidates(ctx context.Context, client *Client, teamID string, me int64, st *storeState, since, start time.Time) (list []candidate, resume time.Time, partial bool, err error) {
	byID := map[string]int{}
	add := func(t Task, rank int, assigned bool) {
		if !validTaskID(t.ID) {
			return
		}
		if i, ok := byID[t.ID]; ok {
			c := &list[i]
			c.assigned = c.assigned || assigned
			if rank < c.rank {
				c.rank = rank
			}
			if c.task.Name == "" {
				c.task.Name = t.Name
			}
			return
		}
		byID[t.ID] = len(list)
		list = append(list, candidate{task: t, rank: rank, assigned: assigned})
	}

	// Oldest first: if the listing is cut, everything up to the last task returned was listed.
	updated, truncU, err := client.Tasks(ctx, teamID, TaskQuery{UpdatedAfter: since, IncludeClosed: true, MaxPages: maxUpdatedPages, OldestFirst: true})
	if err != nil {
		return nil, time.Time{}, false, err
	}
	for _, t := range updated {
		add(t, 0, t.IsAssignedTo(me))
	}
	if truncU && len(updated) > 0 {
		// 1 ms before the last listed task, so tasks sharing its update date are listed again
		// (already-seen comments are skipped); never at or before since, so the window always moves.
		resume = parseMillis(updated[len(updated)-1].DateUpdated).Add(-time.Millisecond)
		if !resume.After(since) {
			resume = since.Add(time.Millisecond)
		}
	}

	mine, truncA, err := client.Tasks(ctx, teamID, TaskQuery{UpdatedAfter: start.Add(-assignedWindow), AssigneeID: me, MaxPages: maxAssignedPages})
	if err != nil {
		return nil, time.Time{}, false, err
	}
	for _, t := range mine {
		add(t, 1, true)
	}

	for id, w := range st.Watched {
		add(Task{ID: id, Name: w.Name}, 2, false)
	}
	for id, cur := range st.NextSince {
		add(Task{ID: id, Name: cur.Name}, 3, false)
	}

	for i := range list {
		c := &list[i]
		_, c.watched = st.Watched[c.task.ID]
		c.since = since
		if cur, ok := st.NextSince[c.task.ID]; ok && cur.Since.Before(c.since) {
			c.since = cur.Since
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if !a.since.Equal(b.since) {
			return a.since.Before(b.since)
		}
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		return a.task.ID < b.task.ID
	})
	return list, resume, truncA, nil
}

type threadUpdate struct {
	id    string
	state ThreadState
}

// scanTask reads one task's comments (more pages when the newest one does not reach back to the
// window or a thread I am in fell off it) and the threads that changed, then records the results.
// complete is false when something was left for the next poll; state is only touched once the
// task was read, so a failure leaves everything to the next poll.
func (p *Poller) scanTask(ctx context.Context, client *Client, me int64, c candidate, st *storeState, start time.Time, res *PollResult, fresh *[]Mention) (complete bool, err error) {
	complete = true
	page, err := client.TaskComments(ctx, c.task.ID)
	if err != nil {
		return false, err
	}

	// Threads I am in that should be on this task; they must be found to keep checking their replies.
	want := map[string]bool{}
	for id, ts := range st.Threads {
		if ts.TaskID == c.task.ID && ts.Participated {
			want[id] = true
		}
	}
	seenIDs := map[string]bool{}
	var comments []Comment
	addPage := func(pg []Comment) {
		for _, cm := range pg {
			if !seenIDs[cm.ID] {
				seenIDs[cm.ID] = true
				comments = append(comments, cm)
				delete(want, cm.ID)
			}
		}
	}
	addPage(page)
	for pages := 1; len(page) >= CommentPageSize; pages++ {
		oldest := page[len(page)-1]
		olderMayBeNew := oldest.Time().After(c.since)
		if !olderMayBeNew && len(want) == 0 {
			break
		}
		limit := maxCommentPagesThreads
		if olderMayBeNew {
			limit = maxCommentPagesNew
		}
		if pages >= limit {
			if olderMayBeNew {
				complete = false // new comments beyond what we read: resume from this task's cursor
			}
			break
		}
		if page, err = client.TaskCommentsBefore(ctx, c.task.ID, &oldest); err != nil {
			return false, err
		}
		addPage(page)
	}

	act := TaskActivity{
		Task: c.task, AssignedToMe: c.assigned, Comments: comments,
		Replies: map[string][]Comment{}, ReplySince: map[string]time.Time{},
	}
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
			complete = false
			break
		}
		replies, err := client.Replies(ctx, root.ID)
		if errors.Is(err, ErrNotFound) {
			delete(st.Threads, root.ID)
			continue
		}
		if err != nil {
			return false, err
		}
		fetched++
		act.Replies[root.ID] = replies
		if ok {
			act.ReplySince[root.ID] = known.CheckedAt.Add(-pollOverlap)
		}
		updates = append(updates, threadUpdate{id: root.ID, state: ThreadState{
			TaskID:       c.task.ID,
			ReplyCount:   root.ReplyCount,
			Participated: known.Participated,
			CheckedAt:    start,
		}})
	}

	// Threads I am in whose root fell beyond the pages read (very long tasks): read their replies
	// directly, so they keep being checked. The root is not needed: the thread is known to be mine.
	for _, id := range sortedKeys(want) {
		if fetched >= maxThreadsPerTask {
			complete = false
			break
		}
		known := st.Threads[id]
		replies, err := client.Replies(ctx, id)
		if errors.Is(err, ErrNotFound) {
			delete(st.Threads, id)
			continue
		}
		if err != nil {
			return false, err
		}
		fetched++
		if len(replies) == known.ReplyCount {
			st.Threads[id] = ThreadState{TaskID: c.task.ID, ReplyCount: known.ReplyCount, Participated: true, CheckedAt: start}
			continue
		}
		// A stand-in root authored by me: Detect then treats the replies as replies in my thread.
		act.Comments = append(act.Comments, Comment{ID: id, User: User{ID: me}, ReplyCount: len(replies)})
		act.Replies[id] = replies
		act.ReplySince[id] = known.CheckedAt.Add(-pollOverlap)
		updates = append(updates, threadUpdate{id: id, state: ThreadState{
			TaskID: c.task.ID, ReplyCount: len(replies), Participated: true, CheckedAt: start,
		}})
	}

	dr := Detect(me, act, c.since)

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
			upsertMention(st, m)
			continue
		}
		*fresh = append(*fresh, m) // deliver() writes it to the note and stores it
	}
	res.Checked++
	return complete, nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// isFatal reports errors that make the rest of the poll pointless.
func isFatal(err error) bool {
	var rl *RateLimitError
	return errors.Is(err, ErrUnauthorized) || errors.As(err, &rl) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

var taskIDRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func validTaskID(id string) bool { return taskIDRegex.MatchString(id) }
