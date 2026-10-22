package calendar

import (
	"time"
	"os"
	"strings"
	"testing"
)

func TestCalendarManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "calendar_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr := NewManager(tempDir)

	testDate := "2026-10-08"

	// 1. Initial GetDayNote returns template
	note, err := mgr.GetDayNote(testDate)
	if err != nil {
		t.Fatalf("GetDayNote failed: %v", err)
	}
	if note.Exists {
		t.Errorf("expected exists=false for new note, got true")
	}
	if !strings.Contains(note.Content, testDate) {
		t.Errorf("expected date in initial template, got: %s", note.Content)
	}

	// 2. Save custom note
	customContent := "# Custom Day Note\n- [x] Done something"
	if err := mgr.SaveDayNote(testDate, customContent); err != nil {
		t.Fatalf("SaveDayNote failed: %v", err)
	}

	savedNote, err := mgr.GetDayNote(testDate)
	if err != nil {
		t.Fatalf("GetDayNote after save failed: %v", err)
	}
	if !savedNote.Exists {
		t.Errorf("expected exists=true, got false")
	}
	if savedNote.Content != customContent {
		t.Errorf("expected %q, got %q", customContent, savedNote.Content)
	}

	// 3. Add Handoff item
	handoff := HandoffItem{
		Date:     testDate,
		BubbleID: "aditum-tar-842",
		TaskName: "Aditum (TAR-842)",
		Agent:    "claude",
		Status:   "in_progress",
		Summary:  "Completed interest calculation logic",
		Pending:  "Run liquibase migrations and test endpoint",
	}

	created, err := mgr.AddHandoff(handoff, "")
	if err != nil {
		t.Fatalf("AddHandoff failed: %v", err)
	}
	if created.ID == "" {
		t.Errorf("expected non-empty ID")
	}

	// Verify markdown was appended
	updatedNote, err := mgr.GetDayNote(testDate)
	if err != nil {
		t.Fatalf("GetDayNote after handoff failed: %v", err)
	}
	if len(updatedNote.Handoffs) != 1 {
		t.Fatalf("expected 1 handoff, got %d", len(updatedNote.Handoffs))
	}
	if !strings.Contains(updatedNote.Content, "TAR-842") {
		t.Errorf("expected handoff block in note content, got: %s", updatedNote.Content)
	}

	// 4. Month Summary
	summary, err := mgr.GetMonthSummary(2026, 10)
	if err != nil {
		t.Fatalf("GetMonthSummary failed: %v", err)
	}
	if len(summary) != 1 {
		t.Fatalf("expected 1 day summary, got %d", len(summary))
	}
	if summary[0].Date != testDate {
		t.Errorf("expected date %s, got %s", testDate, summary[0].Date)
	}
	if summary[0].HandoffCount != 1 {
		t.Errorf("expected handoffCount 1, got %d", summary[0].HandoffCount)
	}

	// 5. Standup Summary
	standup, err := mgr.GetStandup("2026-10-09")
	if err != nil {
		t.Fatalf("GetStandup failed: %v", err)
	}
	if standup.ReferenceDate != testDate {
		t.Errorf("expected reference date %s, got %s", testDate, standup.ReferenceDate)
	}
	if len(standup.PendingItems) != 1 {
		t.Fatalf("expected 1 pending item, got %d", len(standup.PendingItems))
	}
	if standup.PendingItems[0].Pending != handoff.Pending {
		t.Errorf("expected pending text %q, got %q", handoff.Pending, standup.PendingItems[0].Pending)
	}

	// 6. Test GenerateDraft
	draft, err := mgr.GenerateDraft("aditum-tar-842", "Aditum (TAR-842)", "", "claude", "\x1b[32mCompleted task 123\x1b[0m\nNext step is testing")
	if err != nil {
		t.Fatalf("GenerateDraft failed: %v", err)
	}
	if draft.Agent != "claude" {
		t.Errorf("expected agent claude, got %s", draft.Agent)
	}
	if !strings.Contains(draft.Summary, "Completed task 123") {
		t.Errorf("expected summary to contain terminal text, got: %s", draft.Summary)
	}

	// 7. Test UpdateHandoff
	updatedData := *created
	updatedData.Summary = "Updated summary description"
	updatedData.Status = "completed"
	updatedRes, err := mgr.UpdateHandoff(updatedData)
	if err != nil {
		t.Fatalf("UpdateHandoff failed: %v", err)
	}
	if updatedRes.Summary != "Updated summary description" {
		t.Errorf("expected updated summary, got: %s", updatedRes.Summary)
	}
	if updatedRes.Status != "completed" {
		t.Errorf("expected status completed, got: %s", updatedRes.Status)
	}

	// Check markdown reflects update
	noteAfterUpdate, _ := mgr.GetDayNote(testDate)
	if !strings.Contains(noteAfterUpdate.Content, "Updated summary description") {
		t.Errorf("expected updated text in markdown note")
	}

	// 8. Test DeleteHandoff
	err = mgr.DeleteHandoff(testDate, created.ID)
	if err != nil {
		t.Fatalf("DeleteHandoff failed: %v", err)
	}

	// 9. Test AddDailyTodo
	todoNote, err := mgr.AddDailyTodo(testDate, "Check test credentials", false)
	if err != nil {
		t.Fatalf("AddDailyTodo failed: %v", err)
	}
	if !strings.Contains(todoNote.Content, "- [ ] Check test credentials") {
		t.Errorf("expected todo in note content, got: %s", todoNote.Content)
	}

	// 10. Test AppendDailyNote
	appendedNote, err := mgr.AppendDailyNote(testDate, "- Extra observation about API")
	if err != nil {
		t.Fatalf("AppendDailyNote failed: %v", err)
	}
	if !strings.Contains(appendedNote.Content, "- Extra observation about API") {
		t.Errorf("expected observation in note content, got: %s", appendedNote.Content)
	}
}

func TestGetEffectiveHandoffDate(t *testing.T) {
	loc := time.Local

	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		// Tuesday early morning -> Monday
		{
			name:     "Tuesday 02:00 AM shifts to Monday",
			input:    time.Date(2026, time.October, 20, 2, 0, 0, 0, loc), // Tuesday
			expected: "2026-10-19",
		},
		// Wednesday early morning -> Tuesday
		{
			name:     "Wednesday 00:00:00 shifts to Tuesday",
			input:    time.Date(2026, time.October, 21, 0, 0, 0, 0, loc), // Wednesday
			expected: "2026-10-20",
		},
		// Thursday early morning boundary -> Wednesday
		{
			name:     "Thursday 04:59:59 shifts to Wednesday",
			input:    time.Date(2026, time.October, 22, 4, 59, 59, 0, loc), // Thursday
			expected: "2026-10-21",
		},
		// Friday early morning -> Thursday
		{
			name:     "Friday 03:30 AM shifts to Thursday",
			input:    time.Date(2026, time.October, 23, 3, 30, 0, 0, loc), // Friday
			expected: "2026-10-22",
		},
		// Saturday early morning -> Friday (skips weekend)
		{
			name:     "Saturday 01:00 AM shifts to Friday",
			input:    time.Date(2026, time.October, 24, 1, 0, 0, 0, loc), // Saturday
			expected: "2026-10-23",
		},
		// Sunday early morning -> Friday (skips weekend)
		{
			name:     "Sunday 03:00 AM shifts to Friday",
			input:    time.Date(2026, time.October, 25, 3, 0, 0, 0, loc), // Sunday
			expected: "2026-10-23",
		},
		// Monday early morning -> Friday (skips weekend)
		{
			name:     "Monday 02:00 AM shifts to Friday",
			input:    time.Date(2026, time.October, 26, 2, 0, 0, 0, loc), // Monday
			expected: "2026-10-23",
		},
		// Exactly 05:00 AM cutoff: stays on current day
		{
			name:     "Tuesday 05:00:00 AM cutoff stays on Tuesday",
			input:    time.Date(2026, time.October, 20, 5, 0, 0, 0, loc), // Tuesday
			expected: "2026-10-20",
		},
		// Afternoon: stays on current day
		{
			name:     "Wednesday 02:30 PM stays on Wednesday",
			input:    time.Date(2026, time.October, 21, 14, 30, 0, 0, loc), // Wednesday
			expected: "2026-10-21",
		},
		// Late evening: stays on current day
		{
			name:     "Friday 11:59:59 PM stays on Friday",
			input:    time.Date(2026, time.October, 23, 23, 59, 59, 0, loc), // Friday
			expected: "2026-10-23",
		},
		// Saturday afternoon stays on Saturday
		{
			name:     "Saturday 02:00 PM stays on Saturday",
			input:    time.Date(2026, time.October, 24, 14, 0, 0, 0, loc), // Saturday
			expected: "2026-10-24",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEffectiveHandoffDate(tt.input)
			if got != tt.expected {
				t.Errorf("GetEffectiveHandoffDate(%v) = %s, expected %s", tt.input, got, tt.expected)
			}
		})
	}
}

