package calendar

import (
	"fmt"
	"os"
	"path/filepath"
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

// A note that exists but cannot be read must not be replaced by the empty template.
func TestUnreadableNoteIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	const date = "2026-11-26"
	// A directory where the note file should be: reading it fails with an error other than "not exist".
	blocker := filepath.Join(dir, date+".md")
	if err := os.Mkdir(blocker, 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := mgr.AddDailyTodoUnique(date, "x", "clickup:1"); err == nil {
		t.Error("AddDailyTodoUnique should fail on an unreadable note")
	}
	if _, err := mgr.AddDailyTodo(date, "x", false); err == nil {
		t.Error("AddDailyTodo should fail on an unreadable note")
	}
	if _, err := mgr.AppendDailyNote(date, "x"); err == nil {
		t.Error("AppendDailyNote should fail on an unreadable note")
	}
	if fi, err := os.Stat(blocker); err != nil || !fi.IsDir() {
		t.Errorf("the unreadable entry was replaced: %v", err)
	}
}

// Saving an edit keeps tagged lines added by background jobs after the editor loaded the note,
// atomically with the write, while lines the user deleted stay deleted.
func TestSaveDayNoteMergingKeepsConcurrentLines(t *testing.T) {
	mgr := NewManager(t.TempDir())
	const date = "2026-11-26"
	if _, err := mgr.AddDailyTodoUnique(date, "viejo", "clickup:1"); err != nil {
		t.Fatal(err)
	}
	loaded, err := mgr.GetDayNote(date)
	if err != nil {
		t.Fatal(err)
	}
	// While the user edits, the poller adds two more lines.
	for _, id := range []string{"2", "3"} {
		if _, err := mgr.AddDailyTodoUnique(date, "nuevo "+id, "clickup:"+id); err != nil {
			t.Fatal(err)
		}
	}
	// The user deleted "viejo" and wrote a manual item.
	edited := strings.Replace(loaded.Content, "- [ ] viejo <!-- clickup:1 -->", "- [ ] mi tarea", 1)

	merged, err := mgr.SaveDayNoteMerging(date, edited, loaded.Content)
	if err != nil || merged != 2 {
		t.Fatalf("merged=%d err=%v", merged, err)
	}
	note, _ := mgr.GetDayNote(date)
	for _, want := range []string{"nuevo 3 <!-- clickup:3 -->", "nuevo 2 <!-- clickup:2 -->", "- [ ] mi tarea"} {
		if !strings.Contains(note.Content, want) {
			t.Errorf("missing %q:\n%s", want, note.Content)
		}
	}
	if strings.Contains(note.Content, "clickup:1") {
		t.Errorf("a line deleted by the user came back:\n%s", note.Content)
	}
	if strings.Index(note.Content, "clickup:3") > strings.Index(note.Content, "clickup:2") {
		t.Errorf("carried lines should keep their order (newest first):\n%s", note.Content)
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
