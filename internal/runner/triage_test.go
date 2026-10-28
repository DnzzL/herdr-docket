package runner

import (
	"strings"
	"testing"
	"time"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/host"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// triage is a triage card run by a queue's triager in a queue that also has
// a verifier: the run must stop at the triager's move, with no stage after it.
type triage struct {
	host         *fakeHost
	board        *fakeBoard
	runner       *Runner
	verifierRuns int
}

func newTriage(t *testing.T, move func(b *fakeBoard)) *triage {
	t.Helper()
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	tr := &triage{host: &fakeHost{}, board: &fakeBoard{items: map[string]work.Task{
		"TASK-1": {ID: "TASK-1", Title: "Voice notes", Phase: "Triage", Open: true, Triage: true, Assignee: "dev",
			Notes: "## Round 1\n1. Where does the audio go?\n   A. attached (reco)\n\n## Answer 1\n1.A"},
	}}}
	tr.runner = New(tr.host, fleet.Settings{Dir: "/fleet", Source: fleet.SourceConfig{Verifier: "rev", Triager: "tri"}})
	tr.runner.sleep = func(time.Duration) {}
	tr.runner.agents = func() map[string]fleet.Agent {
		return map[string]fleet.Agent{"rev": {Name: "rev", Workdir: "/w", Workspace: "root", Kind: "claude", TimeoutMinutes: 1, Persona: "R"}}
	}
	tr.host.after = func() {
		if strings.Contains(tr.host.spec.Prompt, "task verdict") {
			tr.verifierRuns++
			return
		}
		move(tr.board)
	}
	return tr
}

func (tr *triage) run() error {
	t := tr.board.items["TASK-1"]
	return tr.runner.Run(tr.board, t, fleet.Agent{Name: "tri", Workdir: "/w", Workspace: "root", Kind: "claude",
		TimeoutMinutes: 1, Persona: "P", RoleBrief: "TRIAGER ROLE"}, history.TriggerPoll)
}

// ask is what a triager does when a question is still open: the card goes
// to Question, waiting on the owner.
func ask(b *fakeBoard) {
	it := b.items["TASK-1"]
	it.Triage, it.Question, it.Phase = false, true, string(work.PhaseQuestion)
	b.items["TASK-1"] = it
}

func TestATriageRunGivesTheTriagerItsRoleAndTheCardsRounds(t *testing.T) {
	tr := newTriage(t, ask)
	if err := tr.run(); err != nil {
		t.Fatal(err)
	}
	p := tr.host.spec.Prompt
	for _, want := range []string{"TRIAGER ROLE", "## Round 1", "## Answer 1", "task ask TASK-1"} {
		if !strings.Contains(p, want) {
			t.Errorf("triage prompt lacks %q:\n%s", want, p)
		}
	}
	if strings.Contains(p, "task done TASK-1") {
		t.Errorf("a triage run publishes or asks, it never closes the card:\n%s", p)
	}
	// The card's status is the grill's state: claiming it In Progress would
	// take it out of triage before the triager has said anything.
	for _, ph := range tr.board.phaseSeen {
		if ph == work.PhaseInProgress {
			t.Fatal("a triage run must not move the card to In Progress")
		}
	}
	runs, err := history.Runs("TASK-1", 1)
	if err != nil || len(runs) != 1 || runs[0].Trigger != history.TriggerTriage || runs[0].Status != history.StatusDone {
		t.Fatalf("history = %+v, %v", runs, err)
	}
}

func TestATriageRunIsNeverFollowedByTheVerifier(t *testing.T) {
	tr := newTriage(t, func(b *fakeBoard) {
		it := b.items["TASK-1"]
		it.Triage, it.Phase, it.Assignee = false, string(work.PhaseTodo), "dev"
		b.items["TASK-1"] = it
		// A pull request recorded on the run must not pull it into the pipeline.
		history.SetPullRequest("TASK-1", prURL)
	})
	if err := tr.run(); err != nil {
		t.Fatal(err)
	}
	if tr.verifierRuns != 0 || len(tr.host.specs) != 1 {
		t.Fatalf("verifier runs %d, provisions %d: a published card goes back to the queue, not the pipeline", tr.verifierRuns, len(tr.host.specs))
	}
	if it := tr.board.items["TASK-1"]; !it.Open || it.Phase != string(work.PhaseTodo) {
		t.Fatalf("the published card must stand as the triager left it: %+v", it)
	}
}

func TestATriageRunThatLeavesTheCardInTriageFails(t *testing.T) {
	tr := newTriage(t, func(*fakeBoard) {})
	err := tr.run()
	if err == nil || !strings.Contains(err.Error(), "triage") {
		t.Fatalf("err = %v", err)
	}
	runs, herr := history.Runs("TASK-1", 1)
	if herr != nil || len(runs) != 1 || runs[0].Status != history.StatusFailed || runs[0].Trigger != history.TriggerTriage {
		t.Fatalf("history = %+v, %v", runs, herr)
	}
	if tr.board.items["TASK-1"].Verdict != work.Failed || !hasNoteContaining(tr.board, "triage") {
		t.Fatalf("the card must close Failed saying why: %+v, notes %v", tr.board.items["TASK-1"], tr.board.notes)
	}
	if len(tr.host.notifies) != 1 {
		t.Fatalf("a failed triage run must reach a human: %+v", tr.host.notifies)
	}
}

// The triager edits the card through the tracker's own CLI, and the daemon
// reads the queue from the repo itself: a worktree copy would take the edit
// where nothing reads it. So a triage run works in the repo root whatever
// the agent's workspace says.
func TestATriageRunWorksInTheRepoRootEvenForAWorktreeAgent(t *testing.T) {
	tr := newTriage(t, ask)
	it := tr.board.items["TASK-1"]
	if err := tr.runner.Run(tr.board, it, fleet.Agent{Name: "tri", Workdir: "/w", Workspace: "worktree", Kind: "claude",
		TimeoutMinutes: 1, Persona: "P"}, history.TriggerPoll); err != nil {
		t.Fatal(err)
	}
	if got := tr.host.spec.Workspace; got != host.WorkspaceRoot || tr.host.spec.Repo != "/w" {
		t.Fatalf("triage run workspace = %q in %q, want root in /w", got, tr.host.spec.Repo)
	}
}

// The owner may answer the round the triager just asked before the run has
// settled: the card is back in Triage, but with an answer it did not have
// when the run started — handed on twice, not left untouched.
func TestATriageCardAnsweredDuringTheRunIsHandedOnNotFailed(t *testing.T) {
	tr := newTriage(t, func(b *fakeBoard) {
		it := b.items["TASK-1"]
		it.Notes += "\n\n## Round 2\n1. Keep it how long?\n\n## Answer 2\n1. a month"
		b.items["TASK-1"] = it // asked, then answered: in Triage again
	})
	if err := tr.run(); err != nil {
		t.Fatalf("a card answered mid-run is not a failed run: %v", err)
	}
	if it := tr.board.items["TASK-1"]; !it.Open || it.Verdict != "" {
		t.Fatalf("the answered card must stay in triage for the next run: %+v", it)
	}
}
