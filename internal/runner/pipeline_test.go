package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/gate"
	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/host"
	"github.com/DnzzL/herdr-docket/internal/work"
)

const prURL = "https://github.com/o/r/pull/7"

type fakeForge struct {
	// states are answered in order, the last one repeating: a PR whose CI is
	// pending, then green.
	states []gate.PR
	reads  int
	merged []string
	// labels is the PR's own state, the label operations applied in order —
	// so "a merged PR never carries merge-needed" is read off the PR, not off
	// the calls that happened to touch it.
	labels []string
	// owners is what the forge serves as the base branch's CODEOWNERS rule
	// patterns, nil meaning no file on the base; ownerAsked counts how often
	// the gate came for it.
	owners     []string
	ownerAsked int
}

func (f *fakeForge) PR(string) (gate.PR, error) {
	s := f.states[min(f.reads, len(f.states)-1)]
	f.reads++
	return s, nil
}
func (f *fakeForge) Merge(url, head string) error {
	f.merged = append(f.merged, url+"@"+head)
	return nil
}
func (f *fakeForge) Comment(string, string) error { return nil }
func (f *fakeForge) AddLabel(_ string, label string) error {
	f.labels = append(f.labels, label)
	return nil
}
func (f *fakeForge) RemoveLabel(_ string, label string) error {
	kept := f.labels[:0]
	for _, l := range f.labels {
		if l != label {
			kept = append(kept, l)
		}
	}
	f.labels = kept
	return nil
}

// owners are what the forge serves as the PR's base branch's CODEOWNERS —
// nil is no file on the base.
func (f *fakeForge) Codeowners(string) ([]string, error) {
	f.ownerAsked++
	return f.owners, nil
}

func greenPR() gate.PR {
	return gate.PR{HeadSHA: "h1", PatchID: "p1", Checks: gate.ChecksPass, Files: []string{"main.go"}, State: "OPEN"}
}

// pipeline is a queue with a verifier, scripted run by run: the worker's runs
// deliver the PR, the verifier's record the verdicts in order.
type pipeline struct {
	t        *testing.T
	host     *fakeHost
	board    *fakeBoard
	forge    *fakeForge
	runner   *Runner
	verdicts []string
	// owners is what the forge serves as the base branch's CODEOWNERS rule
	// patterns, nil meaning no file on the base.
	owners     []string
	ownerAsked int
	// workerDelivers false makes the worker close the task itself, no PR.
	workerDelivers bool
	workerRuns     int
	verifierRuns   int
	prompts        []string
	slept          int
	// authorWorkdir is where the author's checkout lives — a test that wants
	// files in it points the field here.
	authorWorkdir string
}

func newPipeline(t *testing.T, merge string, verdicts ...string) *pipeline {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	p := &pipeline{t: t, host: &fakeHost{}, board: newBoard("TASK-1"), forge: &fakeForge{states: []gate.PR{greenPR()}}, verdicts: verdicts, workerDelivers: true}
	p.runner = New(p.host, fleet.Settings{Dir: "/fleet", Source: fleet.SourceConfig{Verifier: "rev", Merge: merge}})
	p.runner.forge = p.forge
	p.runner.sleep = func(time.Duration) { p.slept++ }
	p.runner.agents = func() map[string]fleet.Agent {
		return map[string]fleet.Agent{"rev": {Name: "rev", Workdir: "/w", Workspace: "root", Kind: "claude", TimeoutMinutes: 1, Persona: "R"}}
	}
	p.host.after = func() {
		p.prompts = append(p.prompts, p.host.spec.Prompt)
		if strings.Contains(p.host.spec.Prompt, "task verdict") {
			v := p.verdicts[p.verifierRuns]
			p.verifierRuns++
			if v != "" {
				if err := history.SetVerification("TASK-1", v, "p1"); err != nil {
					t.Fatal(err)
				}
				p.board.Comment("TASK-1", "verdict "+v)
			}
			return
		}
		p.workerRuns++
		if !p.workerDelivers {
			p.board.Close("TASK-1", work.Done)
			return
		}
		if err := history.SetPullRequest("TASK-1", prURL); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func (p *pipeline) run() error {
	p.t.Helper()
	p.forge.owners = p.owners // the fake maps the pipeline's answer, at run time
	if p.authorWorkdir == "" {
		p.authorWorkdir = "/w"
	}
	return p.runner.Run(p.board, work.Task{ID: "TASK-1", Title: "T", Open: true},
		fleet.Agent{Name: "dev", Workdir: p.authorWorkdir, Workspace: "worktree", Kind: "claude", TimeoutMinutes: 1, Persona: "P"}, "manual")
}

func TestAPassOnAGreenPRMergesAndClosesDone(t *testing.T) {
	p := newPipeline(t, "auto", gate.Pass)
	if err := p.run(); err != nil {
		t.Fatal(err)
	}
	if len(p.forge.merged) != 1 || p.forge.merged[0] != prURL+"@h1" {
		t.Fatalf("merged = %v, want the PR merged at the head the gate saw", p.forge.merged)
	}
	if p.board.open("TASK-1") || p.board.verdict("TASK-1") != work.Done {
		t.Fatalf("task = %+v, want closed done", p.board.items["TASK-1"])
	}
	if p.workerRuns != 1 || p.verifierRuns != 1 {
		t.Fatalf("runs: worker %d, verifier %d", p.workerRuns, p.verifierRuns)
	}
	if !hasNoteContaining(p.board, "merged") || len(p.host.notifies) != 1 {
		t.Fatalf("notes %v, notifies %v: a merge is said on the task and to a human", p.board.notes, p.host.notifies)
	}
	if len(p.forge.labels) != 0 {
		t.Errorf("labels = %v: the gate never marks what it merges itself", p.forge.labels)
	}
}

// TASK-63 #2 (second half): a label from an earlier hold is a debt the merge
// pays. Whatever the PR carried when the gate got to it, it must not carry
// afterwards — a merged PR never reads as owed.
func TestTheGateClearsTheLabelOffAPRItMerges(t *testing.T) {
	p := newPipeline(t, "auto", gate.Pass)
	p.forge.labels = []string{"merge-needed"} // left on by a hold before this run
	if err := p.run(); err != nil {
		t.Fatal(err)
	}
	if len(p.forge.merged) != 1 {
		t.Fatalf("merged = %v, want the PR merged", p.forge.merged)
	}
	if len(p.forge.labels) != 0 {
		t.Errorf("labels = %v, want the merged PR carrying nothing", p.forge.labels)
	}
}

// A FAIL sends the worker back with the verifier's notes and its own PR; the
// second verdict decides.
func TestAFailSendsTheWorkerBackThenAPassMerges(t *testing.T) {
	p := newPipeline(t, "auto", gate.Fail, gate.Pass)
	if err := p.run(); err != nil {
		t.Fatal(err)
	}
	if p.workerRuns != 2 || p.verifierRuns != 2 || len(p.forge.merged) != 1 {
		t.Fatalf("worker %d, verifier %d, merged %v", p.workerRuns, p.verifierRuns, p.forge.merged)
	}
	rework := p.prompts[2]
	if !strings.Contains(rework, prURL) {
		t.Fatalf("the rework prompt must name the PR it fixes:\n%s", rework)
	}
}

func TestTwoFailsBlockTheTaskForAHuman(t *testing.T) {
	p := newPipeline(t, "auto", gate.Fail, gate.Fail)
	if err := p.run(); err != nil {
		t.Fatal(err)
	}
	if len(p.forge.merged) != 0 || p.board.verdict("TASK-1") != work.Blocked {
		t.Fatalf("merged %v, verdict %q", p.forge.merged, p.board.verdict("TASK-1"))
	}
	if p.workerRuns != 2 || p.verifierRuns != 2 {
		t.Fatalf("worker %d, verifier %d: two rounds, no third", p.workerRuns, p.verifierRuns)
	}
	if len(p.host.notifies) != 1 {
		t.Fatalf("two rounds and a hold, one popup, saw %v", p.host.notifies)
	}
	n := p.host.notifies[0]
	if n.title != "merge needed — o/r#7" || n.sound != host.SoundRequest {
		t.Errorf("popup = %+v, want 'merge needed — o/r#7' in the request sound", n)
	}
	if !strings.Contains(n.body, "2 times") {
		t.Errorf("body = %q, want the reason it stopped", n.body)
	}
	if len(p.forge.labels) != 1 || p.forge.labels[0] != "merge-needed" {
		t.Errorf("labels = %v, want the PR marked merge-needed", p.forge.labels)
	}
}

func TestACriticalTaskIsHeldWithTheReasonOnIt(t *testing.T) {
	p := newPipeline(t, "auto", gate.Pass)
	it := p.board.items["TASK-1"]
	it.Labels = []string{"critical"}
	p.board.items["TASK-1"] = it
	if err := p.run(); err != nil {
		t.Fatal(err)
	}
	if len(p.forge.merged) != 0 || p.board.verdict("TASK-1") != work.Blocked {
		t.Fatalf("merged %v, verdict %q", p.forge.merged, p.board.verdict("TASK-1"))
	}
	if !hasNoteContaining(p.board, "critical") {
		t.Fatalf("notes %v must say why", p.board.notes)
	}
}

func TestACodeownersPathIsHeld(t *testing.T) {
	p := newPipeline(t, "auto", gate.Pass)
	p.owners = []string{"*.go"}
	if err := p.run(); err != nil {
		t.Fatal(err)
	}
	if len(p.forge.merged) != 0 || !hasNoteContaining(p.board, "main.go") {
		t.Fatalf("merged %v, notes %v", p.forge.merged, p.board.notes)
	}
	// TASK-63 #4 (first case): this stop is the human's — one popup naming
	// the PR they owe, in the sound that says "act", and the PR marked so it
	// can be found again later.
	if len(p.host.notifies) != 1 {
		t.Fatalf("one hold, one popup, saw %v", p.host.notifies)
	}
	n := p.host.notifies[0]
	if n.title != "merge needed — o/r#7" || n.sound != host.SoundRequest {
		t.Errorf("popup = %+v, want 'merge needed — o/r#7' in the request sound", n)
	}
	if !strings.Contains(n.body, "CODEOWNERS") {
		t.Errorf("body = %q, want the reason it stopped", n.body)
	}
	if len(p.forge.labels) != 1 || p.forge.labels[0] != "merge-needed" {
		t.Errorf("labels = %v, want the PR marked merge-needed", p.forge.labels)
	}
}

// The gate's CODEOWNERS is the PR's base branch's, read through the forge:
// the author's checkout on disk is behind or on a branch without the file, so
// asking it would protect whatever its stale copy says — or nothing. The
// forge is named with the PR's url because that is where the base ref is
// known (TASK-57).
func TestTheGateReadsCodeownersFromTheForgeNotTheAuthorsCheckout(t *testing.T) {
	p := newPipeline(t, "auto", gate.Pass)
	p.owners = []string{"main.go"} // a base-branch rule the changed file falls under
	p.authorWorkdir = "/nowhere"   // and the author's checkout does not exist
	if err := p.run(); err != nil {
		t.Fatal(err)
	}
	if p.forge.ownerAsked == 0 {
		t.Fatal("the gate never asked the forge for the base branch's CODEOWNERS")
	}
	if len(p.forge.merged) != 0 || !hasNoteContaining(p.board, "main.go") {
		t.Fatalf("merged %v, notes %v: the base branch's rule must hold the merge", p.forge.merged, p.board.notes)
	}
}

// The gate asks the forge once per pipeline, and a base branch without
// CODEOWNERS leaves nothing protected, whatever sits in the author's
// checkout on disk.
func TestTheGateAsksForBaseCodeownersOnceAndNoFileHoldsNothing(t *testing.T) {
	p := newPipeline(t, "auto", gate.Pass)
	p.owners = nil // the forge: no file on the base
	plain := t.TempDir()
	for _, rel := range []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"} {
		full := filepath.Join(plain, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("*.go @me"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p.authorWorkdir = plain // the local checkout says *.go, the base says nothing
	p.runner.agents = func() map[string]fleet.Agent {
		return map[string]fleet.Agent{"rev": {Name: "rev", Workdir: "/w", Workspace: "root", Kind: "claude", TimeoutMinutes: 1, Persona: "R"}}
	}
	if err := p.run(); err != nil {
		t.Fatal(err)
	}
	if p.forge.ownerAsked != 1 {
		t.Fatalf("the forge was asked %d times, want once per pipeline", p.forge.ownerAsked)
	}
	if p.board.verdict("TASK-1") != work.Done || len(p.forge.merged) != 1 {
		t.Fatalf("verdict %q, merged %v: no file on the base protects nothing",
			p.board.verdict("TASK-1"), p.forge.merged)
	}
}

func TestPendingChecksAreWaitedOn(t *testing.T) {
	p := newPipeline(t, "auto", gate.Pass)
	pending := greenPR()
	pending.Checks = gate.ChecksPending
	p.forge.states = []gate.PR{pending, pending, greenPR()}
	if err := p.run(); err != nil {
		t.Fatal(err)
	}
	if p.slept != 2 || len(p.forge.merged) != 1 {
		t.Fatalf("slept %d, merged %v", p.slept, p.forge.merged)
	}
}

func TestAQueueThatNeverMergesStopsAtAPass(t *testing.T) {
	p := newPipeline(t, "", gate.Pass)
	if err := p.run(); err != nil {
		t.Fatal(err)
	}
	if len(p.forge.merged) != 0 || p.board.verdict("TASK-1") != work.Blocked {
		t.Fatalf("merged %v, verdict %q", p.forge.merged, p.board.verdict("TASK-1"))
	}
	if !hasNoteContaining(p.board, "merge: never") {
		t.Fatalf("notes %v", p.board.notes)
	}
}

// A verifier that ends without a verdict is a run that reported nothing.
func TestAVerifierWithoutAVerdictFailsTheTask(t *testing.T) {
	p := newPipeline(t, "auto", "")
	_ = p.run()
	if len(p.forge.merged) != 0 || p.board.verdict("TASK-1") != work.Failed {
		t.Fatalf("merged %v, verdict %q", p.forge.merged, p.board.verdict("TASK-1"))
	}
}

// A worker that closes its task without a PR — work with no code — never
// reaches the verifier.
func TestAWorkerThatClosesItsTaskSkipsThePipeline(t *testing.T) {
	p := newPipeline(t, "auto", gate.Pass)
	p.workerDelivers = false
	if err := p.run(); err != nil {
		t.Fatal(err)
	}
	if p.verifierRuns != 0 || p.board.verdict("TASK-1") != work.Done {
		t.Fatalf("verifier %d, verdict %q", p.verifierRuns, p.board.verdict("TASK-1"))
	}
}

// While its pipeline runs, a task is spoken for in this process: the daemon
// must not hand it to the worker again between stages.
func TestATaskIsRunningForItsWholePipeline(t *testing.T) {
	p := newPipeline(t, "auto", gate.Pass)
	var seen []bool
	inner := p.host.after
	p.host.after = func() {
		seen = append(seen, p.runner.Running("TASK-1"))
		inner()
	}
	if err := p.run(); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || !seen[0] || !seen[1] || p.runner.Running("TASK-1") {
		t.Fatalf("running during stages = %v, after = %v", seen, p.runner.Running("TASK-1"))
	}
}
