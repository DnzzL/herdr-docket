package main

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/work"
)

func paneFixture() paneData {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	long := strings.Repeat("a title far too long for any phone screen ", 3)
	return paneData{
		Daemon: "daemon down for 4 min",
		Now:    now,
		Queue: []work.Task{
			{ID: "TASK-1", Title: "Which store? " + long, Question: true, Phase: string(work.PhaseQuestion)},
			{ID: "TASK-2", Title: "Parked " + long, Open: true, Blocked: true, Phase: "Blocked"},
			{ID: "TASK-3", Title: "Held " + long, Verdict: work.Blocked, Phase: "Blocked"},
			{ID: "TASK-4", Title: "Plain todo " + long, Open: true, Phase: string(work.PhaseTodo)},
		},
		Runs: []history.Record{
			{RunID: "r9", Task: "TASK-9-with-a-long-id", Agent: "a-long-agent-name", Status: history.StatusRunning, At: now.Add(-3 * time.Minute), WorkspaceID: "workspace-with-a-long-id"},
			{RunID: "r3", Task: "TASK-3", Agent: "verifier", Status: history.StatusDone, At: now.Add(-time.Hour), PullRequest: "https://github.com/o/r/pull/12", Verification: "PASS"},
			{RunID: "r8", Task: "TASK-8", Agent: "dev", Status: history.StatusFailed, At: now.Add(-2 * time.Hour)},
		},
	}
}

func TestPaneNeverWrapsOnANarrowTerminal(t *testing.T) {
	for _, line := range renderPane(paneFixture(), 45) {
		if n := utf8.RuneCountInString(line); n > 45 {
			t.Errorf("%d columns: %q", n, line)
		}
	}
}

func TestPaneOpensOnTheDaemonLine(t *testing.T) {
	lines := renderPane(paneFixture(), 80)
	if len(lines) == 0 || !strings.Contains(lines[0], "daemon down for 4 min") {
		t.Fatalf("first line %q", lines)
	}
}

func TestYourTurnListsQuestionsBlockedAndHeldMerges(t *testing.T) {
	out := strings.Join(renderPane(paneFixture(), 80), "\n")
	turn := strings.Index(out, "Your turn")
	if turn < 0 {
		t.Fatalf("no Your turn section:\n%s", out)
	}
	for _, want := range []string{"? TASK-1 Which store?", "! TASK-2 Parked", "⇡ TASK-3 https://github.com/o/r/pull/12"} {
		i := strings.Index(out, want)
		if i < turn {
			t.Errorf("want %q under Your turn:\n%s", want, out)
		}
	}
	if strings.Contains(out, "TASK-4") {
		t.Errorf("a todo card is not the human's turn:\n%s", out)
	}
	if strings.Contains(out, "! TASK-3") {
		t.Errorf("a held merge is listed once, as a merge:\n%s", out)
	}
	inFlight, settled := strings.Index(out, "TASK-9"), strings.Index(out, "TASK-8")
	if inFlight < turn || settled < inFlight {
		t.Errorf("runs in flight then settled runs come after Your turn:\n%s", out)
	}
}
