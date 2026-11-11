package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Live change notifications for the Cockpit.
//
// Pages used to learn about bubbles and sessions created elsewhere (MCP agents, other tabs, the
// Android app) only through polling. The hub pushes a tiny {"type":"bubbles"}, {"type":"sessions"}
// or {"type":"workspaces"} hint over /ws/events whenever one of them changes, and the page
// re-fetches the list it already knows how to render. Hints carry no data, so a dropped or duplicated one is
// harmless: the next hint, or the page's fallback poll, catches up.
//
// Only lifecycle changes publish hints (create, rename, delete, exit, metadata, dev server state);
// terminal output never does, so typing and streaming output cost nothing here.

const (
	eventKindBubbles    = "bubbles"
	eventKindSessions   = "sessions"
	eventKindWorkspaces = "workspaces"

	// eventCoalesceWindow merges the bursts a single operation produces (creating a bubble rescans
	// worktrees, then spawns its terminal) into one hint per kind.
	eventCoalesceWindow = 100 * time.Millisecond
	// eventSubscriberBuf is the number of hints queued per page before new ones are dropped.
	eventSubscriberBuf = 8

	eventsPingInterval = 30 * time.Second
	eventsPongWait     = 75 * time.Second
	eventsWriteWait    = 10 * time.Second
)

// eventHub fans out coalesced change hints to every connected page.
type eventHub struct {
	mu             sync.Mutex
	window         time.Duration
	subs           map[chan string]struct{}
	pending        map[string]bool
	flushScheduled bool
}

func newEventHub(window time.Duration) *eventHub {
	return &eventHub{
		window:  window,
		subs:    make(map[chan string]struct{}),
		pending: make(map[string]bool),
	}
}

// subscribe registers a page and returns the channel its hints are delivered on.
func (h *eventHub) subscribe() chan string {
	ch := make(chan string, eventSubscriberBuf)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// unsubscribe removes a page. Its channel is not closed: the caller simply stops reading it.
func (h *eventHub) unsubscribe(ch chan string) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}

// publish records that kind changed. It never blocks, so managers can call it while holding their
// own locks. Hints published within the coalescing window go out as one message per kind.
func (h *eventHub) publish(kind string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.subs) == 0 {
		return // nobody is listening; a page that connects later fetches everything anyway
	}
	h.pending[kind] = true
	if !h.flushScheduled {
		h.flushScheduled = true
		time.AfterFunc(h.window, h.flush)
	}
}

func (h *eventHub) flush() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.flushScheduled = false
	for kind := range h.pending {
		msg := `{"type":"` + kind + `"}`
		for ch := range h.subs {
			select {
			case ch <- msg:
			default: // a page that stopped reading just misses this hint
			}
		}
	}
	h.pending = make(map[string]bool)
}

// handleEventsWebSocket streams change hints to a Cockpit page until it disconnects.
func (s *Server) handleEventsWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ch := s.events.subscribe()
	defer s.events.unsubscribe(ch)

	// The page never sends data: the reader only processes pongs and close frames, and notices
	// when the connection goes away.
	done := make(chan struct{})
	conn.SetReadLimit(512)
	_ = conn.SetReadDeadline(time.Now().Add(eventsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(eventsPongWait))
	})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ping := time.NewTicker(eventsPingInterval)
	defer ping.Stop()
	for {
		select {
		case msg := <-ch:
			_ = conn.SetWriteDeadline(time.Now().Add(eventsWriteWait))
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				return
			}
		case <-ping.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(eventsWriteWait)); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}
