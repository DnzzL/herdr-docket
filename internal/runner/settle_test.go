package runner

// The settle pass (TASK-70), through the seam: a held or failed task whose
// pull request the forge has already ended — merged, closed unmerged, still
// open — settled by the same queue and forge the daemon uses, on a cadence
// that is its own rather than the tick's.

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/gate"
	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// settleTaskID is the one task these passes look at, and settlePR the pull
// request the history records for it.
const settleTaskID = "TASK-70"
const settlePR = "https://github.com/DnzzL/herdr-docket/pull/70"

// heldTask is a task parked on a human the way a hold leaves it.
func heldTask() work.Task {
	return work.Task{ID: settleTaskID, Title: "T", Open: false, Blocked: true, Verdict: work.Blocked, Phase: "Blocked"}
}

// failedTask is one a run closed failed, its pull request still out.
func failedTask() work.Task {
	return work.Task{ID: settleTaskID, Title: "T", Open: false, Verdict: work.Failed, Phase: "Failed"}
}

// settlePass runs one settle pass over a queue holding task, with the forge
// answering prState and the task's pull request recorded in history.
func settlePass(t *testing.T, task work.Task, forge *fakeForge) *fakeBoard {
	t.Helper()
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	if err := history.Append(history.Record{RunID: settleTaskID + "-1", Task: settleTaskID, Status: history.StatusDone, PullRequest: settlePR}); err != nil {
		t.Fatalf("record the pull request: %v", err)
	}
	b := newBoard(settleTaskID)
	b.items[settleTaskID] = task
	r := New(&fakeHost{}, fleet.Settings{Dir: "/fleet"})
	r.forge = forge
	r.Settle(b, []work.Task{task})
	return b
}

func TestAHeldTaskWhosePullRequestIsMergedClosesDone(t *testing.T) {
	forge := &fakeForge{states: []gate.PR{{State: "MERGED"}}}
	b := settlePass(t, heldTask(), forge)
	if got := b.verdict(settleTaskID); got != work.Done {
		t.Errorf("verdict = %q, want done: the human merged the pull request", got)
	}
	if len(forge.merged) != 0 {
		t.Errorf("merged %v, want none: the fleet never merges here", forge.merged)
	}
	if len(b.notes) == 0 || !strings.Contains(b.notes[0], "is merged") {
		t.Errorf("notes = %v, want the merge said on the task", b.notes)
	}
}

func TestAFailedTaskWhosePullRequestIsMergedClosesDone(t *testing.T) {
	forge := &fakeForge{states: []gate.PR{{State: "MERGED"}}}
	b := settlePass(t, failedTask(), forge)
	if got := b.verdict(settleTaskID); got != work.Done {
		t.Errorf("verdict = %q, want done: the pull request shipped anyway", got)
	}
	if len(b.notes) == 0 || !strings.Contains(b.notes[0], "is merged") {
		t.Errorf("notes = %v, want the merge said on the task", b.notes)
	}
}

func TestAHeldTaskWhosePullRequestIsClosedUnmergedClosesFailedSayingAHumanClosedIt(t *testing.T) {
	forge := &fakeForge{states: []gate.PR{{State: "CLOSED"}}}
	b := settlePass(t, heldTask(), forge)
	if got := b.verdict(settleTaskID); got != work.Failed {
		t.Errorf("verdict = %q, want failed: a human closed the pull request unmerged", got)
	}
	if len(b.notes) == 0 || !strings.Contains(b.notes[0], "closed unmerged by a human") {
		t.Errorf("notes = %v, want the note to say a human closed it", b.notes)
	}
}

func TestAFailedTaskWhosePullRequestIsClosedUnmergedIsLeftAsItStands(t *testing.T) {
	forge := &fakeForge{states: []gate.PR{{State: "CLOSED"}}}
	b := settlePass(t, failedTask(), forge)
	if got := b.verdict(settleTaskID); got != work.Failed {
		t.Errorf("verdict = %q, want failed left alone", got)
	}
	if len(b.notes) != 0 {
		t.Errorf("notes = %v, want none: the task already says failed", b.notes)
	}
}

func TestAnOpenPullRequestLeavesItsHeldTaskBlocked(t *testing.T) {
	forge := &fakeForge{states: []gate.PR{{State: "OPEN"}}}
	b := settlePass(t, heldTask(), forge)
	if got := b.verdict(settleTaskID); got != work.Blocked {
		t.Errorf("verdict = %q, want the hold standing: the pull request is still open", got)
	}
	if len(b.notes) != 0 {
		t.Errorf("notes = %v, want none: nothing has happened yet", b.notes)
	}
	if forge.reads != 1 {
		t.Errorf("forge reads = %d, want 1: the state was read through the forge", forge.reads)
	}
}

func TestATaskWithNoPullRequestIsNeverReadFromTheForge(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	forge := &fakeForge{states: []gate.PR{{State: "MERGED"}}}
	b := newBoard(settleTaskID)
	task := heldTask()
	b.items[settleTaskID] = task
	r := New(&fakeHost{}, fleet.Settings{Dir: "/fleet"})
	r.forge = forge
	r.Settle(b, []work.Task{task})
	if forge.reads != 0 {
		t.Errorf("forge reads = %d, want none: the task has no pull request", forge.reads)
	}
	if got := b.verdict(settleTaskID); got != work.Blocked {
		t.Errorf("verdict = %q, want the hold standing", got)
	}
}

func TestAnOpenTaskIsNeverSettledEvenWithARecordedPullRequest(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	if err := history.Append(history.Record{RunID: settleTaskID + "-1", Task: settleTaskID, Status: history.StatusDone, PullRequest: settlePR}); err != nil {
		t.Fatalf("record the pull request: %v", err)
	}
	forge := &fakeForge{states: []gate.PR{{State: "MERGED"}}}
	b := newBoard(settleTaskID) // a fresh board: To Do, open
	r := New(&fakeHost{}, fleet.Settings{Dir: "/fleet"})
	r.forge = forge
	r.Settle(b, []work.Task{b.items[settleTaskID]})
	if forge.reads != 0 {
		t.Errorf("forge reads = %d, want none: an open task owes nothing to a forge", forge.reads)
	}
}

func TestAPullRequestThatCannotBeReadLeavesTheTaskAlone(t *testing.T) {
	forge := &fakeForge{err: errors.New("gh: network down")}
	b := settlePass(t, heldTask(), forge)
	if got := b.verdict(settleTaskID); got != work.Blocked {
		t.Errorf("verdict = %q, want the hold standing: the evidence did not settle", got)
	}
	if len(b.notes) != 0 {
		t.Errorf("notes = %v, want none: an unreadable state is a human's", b.notes)
	}
}

// The forge is read every few minutes, not every tick: fifteen seconds of
// daemon ticks spend one read, three minutes of them a second.
func TestThePullRequestIsReadEveryFewMinutesAndNotEveryTick(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	if err := history.Append(history.Record{RunID: settleTaskID + "-1", Task: settleTaskID, Status: history.StatusDone, PullRequest: settlePR}); err != nil {
		t.Fatalf("record the pull request: %v", err)
	}
	forge := &fakeForge{states: []gate.PR{{State: "OPEN"}}}
	b := newBoard(settleTaskID)
	task := heldTask()
	b.items[settleTaskID] = task

	now := time.Now()
	r := New(&fakeHost{}, fleet.Settings{Dir: "/fleet"})
	r.forge = forge
	r.now = func() time.Time { return now }

	// Two minutes of ticks: one read, the pass at daemon start.
	for i := 0; i < 8; i++ {
		r.Settle(b, []work.Task{task})
		now = now.Add(15 * time.Second)
	}
	if forge.reads != 1 {
		t.Errorf("forge reads after 8 ticks = %d, want 1: a tick is not a settle pass", forge.reads)
	}
	// Four more minutes: settleEvery has passed, so one read per pass.
	for i := 0; i < 16; i++ {
		r.Settle(b, []work.Task{task})
		now = now.Add(15 * time.Second)
	}
	if forge.reads != 2 {
		t.Errorf("forge reads after 4 more minutes = %d, want 2: one pass per settleEvery", forge.reads)
	}
}
