package calendar

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestAddDailyTodoUniqueAddsOnce(t *testing.T) {
	mgr := NewManager(t.TempDir())
	const date = "2026-11-26"

	added, err := mgr.AddDailyTodoUnique(date, "💬 **Ana** te mencionó", "clickup:9001")
	if err != nil || !added {
		t.Fatalf("first add: added=%v err=%v", added, err)
	}
	added, err = mgr.AddDailyTodoUnique(date, "💬 **Ana** te mencionó (otra vez)", "clickup:9001")
	if err != nil || added {
		t.Fatalf("second add with the same marker: added=%v err=%v", added, err)
	}

	note, err := mgr.GetDayNote(date)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(note.Content, "<!-- clickup:9001 -->"); n != 1 {
		t.Fatalf("marker appears %d times:\n%s", n, note.Content)
	}
	if !strings.Contains(note.Content, "## 📌 Notas Generales\n- [ ] 💬 **Ana** te mencionó <!-- clickup:9001 -->") {
		t.Errorf("to-do not placed under Notas Generales replacing the placeholder:\n%s", note.Content)
	}
	if strings.Contains(note.Content, "- [ ] \n") {
		t.Errorf("empty placeholder should have been replaced:\n%s", note.Content)
	}
}

func TestAddDailyTodoUniqueRejectsUnsafeMarkers(t *testing.T) {
	mgr := NewManager(t.TempDir())
	for _, bad := range []string{"", "clickup", "clickup:1 -->", "<x>:1", "clickup:a b"} {
		if _, err := mgr.AddDailyTodoUnique("2026-11-26", "x", bad); err == nil {
			t.Errorf("marker %q should be rejected", bad)
		}
	}
}

// Concurrent writers must not drop each other's lines (the old read-then-write lost updates).
func TestConcurrentTodosAreAllKept(t *testing.T) {
	mgr := NewManager(t.TempDir())
	const date = "2026-11-26"
	const n = 40

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			if _, err := mgr.AddDailyTodoUnique(date, fmt.Sprintf("u%d", i), fmt.Sprintf("clickup:%d", i)); err != nil {
				t.Error(err)
			}
		}(i)
		go func(i int) {
			defer wg.Done()
			if _, err := mgr.AddDailyTodo(date, fmt.Sprintf("manual-%d", i), false); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()

	note, err := mgr.GetDayNote(date)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		if !strings.Contains(note.Content, fmt.Sprintf("<!-- clickup:%d -->", i)) {
			t.Errorf("lost unique to-do %d", i)
		}
		if !strings.Contains(note.Content, fmt.Sprintf("- [ ] manual-%d\n", i)) {
			t.Errorf("lost manual to-do %d", i)
		}
	}
}
