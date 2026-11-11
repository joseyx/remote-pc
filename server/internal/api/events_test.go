package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"orbit/internal/bubble"
)

// drain collects every hint that arrives on ch within d.
func drain(ch chan string, d time.Duration) []string {
	var got []string
	deadline := time.After(d)
	for {
		select {
		case msg := <-ch:
			got = append(got, msg)
		case <-deadline:
			return got
		}
	}
}

func TestEventHub_CoalescesBurstsAndFansOut(t *testing.T) {
	h := newEventHub(20 * time.Millisecond)
	a, b := h.subscribe(), h.subscribe()

	for i := 0; i < 5; i++ {
		h.publish(eventKindBubbles)
	}
	h.publish(eventKindSessions)

	for name, ch := range map[string]chan string{"a": a, "b": b} {
		got := drain(ch, 200*time.Millisecond)
		joined := strings.Join(got, ",")
		if len(got) != 2 || !strings.Contains(joined, `{"type":"bubbles"}`) || !strings.Contains(joined, `{"type":"sessions"}`) {
			t.Fatalf("subscriber %s: want one bubbles and one sessions hint, got %v", name, got)
		}
	}

	h.unsubscribe(b)
	h.publish(eventKindBubbles)
	if got := drain(b, 100*time.Millisecond); len(got) != 0 {
		t.Fatalf("unsubscribed page should get nothing, got %v", got)
	}
	if got := drain(a, 100*time.Millisecond); len(got) != 1 {
		t.Fatalf("remaining page should get the new hint, got %v", got)
	}
}

func TestEventHub_SkipsWorkWithoutSubscribers(t *testing.T) {
	h := newEventHub(10 * time.Millisecond)
	h.publish(eventKindBubbles)
	h.mu.Lock()
	scheduled, pending := h.flushScheduled, len(h.pending)
	h.mu.Unlock()
	if scheduled || pending != 0 {
		t.Fatalf("publishing with no pages should not schedule a flush (scheduled=%v pending=%d)", scheduled, pending)
	}
}

func TestEventsWebSocket_PushesBubbleChangesAndRequiresAuth(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/events"

	if _, resp, err := websocket.DefaultDialer.Dial(wsURL+"?token=wrong", nil); err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 with a wrong token, got err=%v resp=%v", err, resp)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?token=test-secret-token", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// The subscription is registered right after the upgrade; give the handler a moment.
	deadline := time.Now().Add(2 * time.Second)
	for {
		srv.events.mu.Lock()
		n := len(srv.events.subs)
		srv.events.mu.Unlock()
		if n > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	start := time.Now()
	srv.bubbleMgr.SetDevRunning("no-such-bubble", false, 0) // any bubble manager mutation notifies

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("expected a hint, got %v", err)
	}
	if string(msg) != `{"type":"bubbles"}` {
		t.Fatalf("unexpected hint %q", msg)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("hint took %v", elapsed)
	}
}

func TestEvents_WorkspaceChangesPublishWorkspacesAndBubbles(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	ch := srv.events.subscribe()
	defer srv.events.unsubscribe(ch)

	if err := srv.bubbleMgr.AddWorkspace(bubble.WorkspaceInfo{ID: "evt-test", Name: "Evt Test", Type: "evt-test", BasePath: t.TempDir()}); err != nil {
		t.Fatalf("AddWorkspace failed: %v", err)
	}
	joined := strings.Join(drain(ch, 500*time.Millisecond), ",")
	if !strings.Contains(joined, `{"type":"workspaces"}`) || !strings.Contains(joined, `{"type":"bubbles"}`) {
		t.Fatalf("adding a workspace should publish workspaces and bubbles hints, got %q", joined)
	}
}
