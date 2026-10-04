package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/work"
)

func TestAssembleCarriesEveryFactTheAgentNeeds(t *testing.T) {
	got := Assemble(
		fleet.Agent{Name: "marketing", Persona: "You run publication."},
		work.Task{
			ID: "TASK-7", Title: "Draft launch post", Open: true,
			Body: "Write the Show HN post.",
			Criteria: []work.Criterion{
				{Index: 1, Text: "under 200 words", Checked: false},
				{Index: 2, Text: "links the repo", Checked: true},
			},
			Notes: "previous attempt stalled",
		},
		"/Users/x/fleet", fleet.Words{Todo: "To Do", Done: "Done", Failed: "Failed", Blocked: "Blocked"},
	)
	for _, want := range []string{
		"You run publication.",
		"TASK-7", "Draft launch post", "Write the Show HN post.",
		"[ ] #1 under 200 words", "[x] #2 links the repo",
		"previous attempt stalled",
		// The verbs, not the invocation: how the CLI is spelled depends on
		// whether a pane's PATH resolves it, and cli_test.go owns that.
		"task note TASK-7",
		"task done TASK-7",
		"--pr",
		"task fail TASK-7",
		"task block TASK-7",
		"task create",
		"-a marketing",
		"one coherent slice",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt is missing %q:\n%s", want, got)
		}
	}
}

// The agent talks to the queue through the fleet's own CLI. It has no reason
// to know the backend, its credentials, or where its files live — and it must
// not be handed the backend CLI again, because that is how the queue gets
// mistaken for Backlog.md.
func TestAssembleNeverNamesTheBackend(t *testing.T) {
	got := Assemble(fleet.Agent{Name: "a", Persona: "P"},
		work.Task{ID: "T-1", Title: "t", Open: true, Body: "b"}, "/d", fleet.Words{})
	for _, absent := range []string{"BACKLOG_CWD", "--check-ac", "--append-notes", "backlog task"} {
		if strings.Contains(got, absent) {
			t.Fatalf("prompt still says %q:\n%s", absent, got)
		}
	}
}

// Criteria are the human's record of what "done" means. The agent reasons
// about them and answers in prose; it does not tick its own boxes.
func TestAssembleTellsTheAgentTheCriteriaAreReadOnly(t *testing.T) {
	got := Assemble(fleet.Agent{Name: "a", Persona: "P"},
		work.Task{ID: "T-1", Title: "t", Open: true,
			Criteria: []work.Criterion{{Index: 1, Text: "works"}}}, "/d", fleet.Words{})
	if !strings.Contains(got, "not yours to edit") {
		t.Fatalf("prompt does not say the criteria are read-only:\n%s", got)
	}
	if !strings.Contains(got, "which criteria you met") {
		t.Fatalf("prompt does not ask for the verdict in the closing note:\n%s", got)
	}
}

func TestAssembleOmitsEmptySections(t *testing.T) {
	got := Assemble(fleet.Agent{Name: "a", Persona: "P"},
		work.Task{ID: "T-1", Title: "t", Open: true}, "/d", fleet.Words{})
	for _, absent := range []string{"Acceptance criteria", "Notes from previous runs", "## Description"} {
		if strings.Contains(got, absent) {
			t.Fatalf("empty section %q rendered:\n%s", absent, got)
		}
	}
}

// A prefixed id means the fleet works several queues. The follow-up command
// has to carry the source, or the agent's next task lands in whichever queue
// happens to be first — so the prompt derives it from the id and the agent
// never has to know a second queue exists.
func TestAssembleRoutesTheFollowUpToTheTasksOwnSource(t *testing.T) {
	got := Assemble(fleet.Agent{Name: "a", Persona: "P"},
		work.Task{ID: "myapp/TASK-12", Title: "t", Open: true}, "/d", fleet.Words{})
	// Pin the whole command, not fragments of it: this is the one line the
	// run is told to type, and the fragments passed even when the command
	// around them changed shape.
	want := `herdr-docket task create "<title>" -d "<what and why>" \
      --ac "<one observable claim per flag, repeatable>" -a a -s myapp`
	if !strings.Contains(got, want) {
		t.Fatalf("follow-up create must read:\n%s\ngot:\n%s", want, got)
	}
}

// A single queue has no prefix and no source to name: the follow-up command
// must stay exactly as it was, with no -s on it.
func TestAssembleLeavesTheFollowUpAloneForAQueueWithNoPrefix(t *testing.T) {
	got := Assemble(fleet.Agent{Name: "a", Persona: "P"},
		work.Task{ID: "TASK-12", Title: "t", Open: true}, "/d", fleet.Words{})
	if strings.Contains(got, " -s ") {
		t.Fatalf("an unprefixed task must not name a source:\n%s", got)
	}
}

// FLEET.md is the company-wide brief: what is true of every agent, written
// once. When it exists its body precedes the persona, so an agent reads the
// shared context before its own.
func TestAssemblePrependsTheFleetBrief(t *testing.T) {
	dir := t.TempDir()
	brief := "We are Acme. Ship small, tell the truth, never touch main."
	if err := os.WriteFile(filepath.Join(dir, "FLEET.md"), []byte(brief+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Assemble(
		fleet.Agent{Name: "dev", Persona: "You are the dev."},
		work.Task{ID: "TASK-1", Title: "t", Open: true},
		dir, fleet.Words{},
	)
	if !strings.Contains(got, brief) {
		t.Fatalf("the brief is missing:\n%s", got)
	}
	if i, j := strings.Index(got, brief), strings.Index(got, "You are the dev."); i < 0 || j < 0 || i > j {
		t.Fatalf("the brief must come before the persona:\n%s", got)
	}
}

// No brief leaves the prompt exactly as it was. This is the byte-for-byte pin:
// the file's absence has to be invisible, so a fleet that never writes one gets
// the same prompt it always did.
func TestAssembleWithoutABriefIsUnchanged(t *testing.T) {
	a := fleet.Agent{Name: "dev", Persona: "You are the dev."}
	v := work.Task{ID: "TASK-1", Title: "t", Open: true, Body: "b"}
	dir := t.TempDir() // no FLEET.md in the dir
	got := Assemble(a, v, dir, fleet.Words{})
	if want := assemble(a, v, "", dir, fleet.Words{}); got != want {
		t.Fatalf("an absent brief changed the prompt:\n got: %q\nwant: %q", got, want)
	}
}

// A scaffold someone opened and never wrote in is no brief at all: an empty
// file must not inject blank lines into every prompt.
func TestAssembleTreatsAnEmptyBriefAsNone(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "FLEET.md"), []byte("\n\n  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := fleet.Agent{Name: "dev", Persona: "You are the dev."}
	v := work.Task{ID: "TASK-1", Title: "t", Open: true}
	if got, want := Assemble(a, v, dir, fleet.Words{}), assemble(a, v, "", dir, fleet.Words{}); got != want {
		t.Fatalf("an empty brief changed the prompt:\n got: %q\nwant: %q", got, want)
	}
}

// init scaffolds a brief that is nothing but comments. Until a human writes
// real prose into it, the scaffold must be invisible — otherwise every agent
// in a fresh fleet opens with init's instructions.
func TestAssembleTreatsAnAllCommentBriefAsNone(t *testing.T) {
	dir := t.TempDir()
	scaffold := "# FLEET.md — the fleet's shared brief\n#\n# Write what is true of the whole fleet here.\n#\n#   We build acme.\n"
	if err := os.WriteFile(filepath.Join(dir, "FLEET.md"), []byte(scaffold), 0o644); err != nil {
		t.Fatal(err)
	}
	a := fleet.Agent{Name: "dev", Persona: "You are the dev."}
	v := work.Task{ID: "TASK-1", Title: "t", Open: true}
	if got, want := Assemble(a, v, dir, fleet.Words{}), assemble(a, v, "", dir, fleet.Words{}); got != want {
		t.Fatalf("a commented scaffold reached the prompt:\n got: %q\nwant: %q", got, want)
	}
}

// The moment a human writes one uncommented line, the whole file becomes the
// brief — comments included, because they are context a human chose to keep.
func TestAssembleSpeaksOnceABriefHasRealProse(t *testing.T) {
	dir := t.TempDir()
	brief := "# FLEET.md\n# Keep this note.\nWe build acme.\n"
	if err := os.WriteFile(filepath.Join(dir, "FLEET.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Assemble(fleet.Agent{Name: "dev", Persona: "You are the dev."},
		work.Task{ID: "TASK-1", Title: "t", Open: true}, dir, fleet.Words{})
	if !strings.Contains(got, "We build acme.") || !strings.Contains(got, "Keep this note.") {
		t.Fatalf("a brief with prose must be prepended whole:\n%s", got)
	}
}

// Three layers, widest first: the fleet's brief, the role's method, then this
// post's own persona. A role is how two agents doing the same job in two
// repos share one method instead of two copies that drift.
func TestTheRoleBriefSitsBetweenTheFleetAndThePersona(t *testing.T) {
	got := assemble(
		fleet.Agent{Name: "dev", Persona: "PERSONA", RoleBrief: "ROLE"},
		work.Task{ID: "TASK-7", Title: "T"},
		"FLEET", "/fleet", fleet.Words{},
	)
	fleetAt, roleAt, personaAt := strings.Index(got, "FLEET"), strings.Index(got, "ROLE"), strings.Index(got, "PERSONA")
	if fleetAt < 0 || roleAt < 0 || personaAt < 0 {
		t.Fatalf("a layer is missing:\n%s", got)
	}
	if !(fleetAt < roleAt && roleAt < personaAt) {
		t.Fatalf("layers out of order (fleet %d, role %d, persona %d)", fleetAt, roleAt, personaAt)
	}
}

// An agent with no role is the whole fleet before roles existed.
func TestNoRoleChangesNothing(t *testing.T) {
	got := assemble(fleet.Agent{Name: "dev", Persona: "PERSONA"}, work.Task{ID: "TASK-7"}, "", "/fleet", fleet.Words{})
	if strings.HasPrefix(strings.TrimSpace(got), "\n") {
		t.Fatal("an absent role must not leave a gap")
	}
	if !strings.HasPrefix(got, "PERSONA") {
		t.Fatalf("want the persona first:\n%s", got[:80])
	}
}

// An agent that edits its project's board writes that project's word, not the
// fleet's idea of it. Before the prompt said them, every persona retyped them
// by hand — and a project that renamed a column left its agent writing a
// status its own CLI refuses.
func TestThePromptNamesTheQueuesOwnWords(t *testing.T) {
	got := assemble(
		fleet.Agent{Name: "pm", Persona: "P"},
		work.Task{ID: "notara/NOT-92", Title: "T"},
		"", "/fleet",
		fleet.Words{Todo: "ready-for-agent", Done: "done", Failed: "ready-for-human", Blocked: "needs-info"},
	)
	for _, want := range []string{"ready-for-agent", "ready-for-human", "needs-info"} {
		if !strings.Contains(got, want) {
			t.Errorf("the prompt must name %q:\n%s", want, got)
		}
	}
	// A project with no word for work in hand is taught none, rather than
	// being handed an empty line to write.
	if strings.Contains(got, "work a run has in hand") {
		t.Error("a phase the project has no word for must not be named")
	}
}

// A backend with no status words — a Basecamp to-do is done or not — is not
// given Backlog.md's vocabulary by accident.
func TestAQueueWithoutWordsIsToldSo(t *testing.T) {
	got := assemble(fleet.Agent{Name: "pm", Persona: "P"}, work.Task{ID: "bc/987"}, "", "/fleet", fleet.Words{})
	if !strings.Contains(got, "no status words") {
		t.Fatalf("want the prompt to say the queue has none:\n%s", got)
	}
	for _, absent := range []string{"To Do", "In Progress"} {
		if strings.Contains(got, absent) {
			t.Errorf("the prompt invented %q for a backend that has none", absent)
		}
	}
}

// contains folds whitespace before searching, so a pinned phrase matches
// however the raw-string in prompt.go happens to be wrapped. The wording is
// pinned; the line breaks are prompt.go's to fluff.
func contains(haystack, needle string) bool {
	return strings.Contains(fold(haystack), fold(needle))
}

// fold collapses every run of whitespace to single spaces, and trims.
func fold(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// The two ways a run can end, side by side: close the task with a verdict
// (done / fail / block) or hand it on to another agent with assign. The old
// heading scored the verb as if only a verdict existed, so a persona whose
// correct ending is a handoff — a dev handing a built PR to a reviewer —
// concluded its run had disobeyed, and ended the turn in prose instead. Both
// endings must be named as endings, neither as an aside.
func TestTwoNamedEndingsAndTheChoiceIsTheTasksToDefer(t *testing.T) {
	got := Assemble(
		fleet.Agent{Name: "dev", Persona: "P"},
		work.Task{ID: "docket/TASK-1", Title: "t", Open: true}, "/d", fleet.Words{},
	)
	if !contains(got, "## Ending the run — required, exactly once") {
		t.Fatalf("the required section must be headed both ways:\n%s", got)
	}
	// "always close" is exactly what lied to a handing-off run.
	if contains(got, "always close what you touch") || contains(got, "always close your task") {
		t.Fatalf("prompt still declares closing the only ending:\n%s", got)
	}
	for _, want := range []string{
		"close the task with a verdict",
		"hand it on to another agent",
		"Decide from the task and your own persona which",
		"the last tool call of the run",
		"a run that ends with a message and no command",
		"task open, agent gone, verdict unreported",
		"Always execute one, and the same ending, and only one",
		"this is the command you cut everything else to run",
		"your task, once, however the run ends",
	} {
		if !contains(got, want) {
			t.Fatalf("prompt is missing %q:\n%s", want, got)
		}
	}
}

// Neither ending is optional-with-nothing. The failure to name was observed
// more than any other: the run does correct work, leaves the task right where
// it sat, and vanishes — that is not a choice, it is the failed run.
func TestNeitherEndingIsOptionalWithNothing(t *testing.T) {
	got := Assemble(fleet.Agent{Name: "a", Persona: "P"},
		work.Task{ID: "T-1", Title: "t", Open: true}, "/d", fleet.Words{})
	if !contains(got, "do not end by leaving it open and unassigned") {
		t.Fatalf("prompt must name 'open and unassigned' as a failure:\n%s", got)
	}
}

// The ordering the reversed runs violated the moment a model read the section
// as "prose, then commands": the ending command runs before the summary word
// is written, and every conclusion the agent draws lands in the note, never
// only in chat.
func TestStopIsNotDoneUntilTheCommandRan(t *testing.T) {
	got := Assemble(fleet.Agent{Name: "a", Persona: "P"},
		work.Task{ID: "docket/TASK-9", Title: "t", Open: true,
			Criteria: []work.Criterion{{Index: 1, Text: "works"}}}, "/d", fleet.Words{})
	if i, j := strings.Index(got, "## Verify,"), strings.Index(got, "## Ending the run"); i < 0 || j < 0 || i > j {
		t.Fatalf("the ending section must sit after the criteria are read:\n%s", got)
	}
	for _, want := range []string{
		"execute it, then summarise",
		"If your run has already said its conclusion,",
		"in the note",
		"not only in chat",
	} {
		if !contains(got, want) {
			t.Fatalf("prompt is missing %q:\n%s", want, got)
		}
	}
}

// verify()'s own required section stated no ordering (TASK-59): a verifier
// could reach its verdict, say it in prose, and stop — the exact run type
// TASK-46's prose-ending failure cites — and the fleet would file the run
// "settled without reporting a verdict". Same pin as
// TestStopIsNotDoneUntilTheCommandRan, one section over.
func TestTheVerdictCommandIsTheLastToolCallOfAVerifyRun(t *testing.T) {
	got := verify(fleet.Agent{Name: "rev", Persona: "R"},
		work.Task{ID: "docket/TASK-59", Title: "t", Open: true},
		"https://github.com/o/r/pull/7", "", "/fleet")
	for _, want := range []string{
		"the last tool call of the run",
		"execute it, then summarise",
		"settled without reporting a verdict",
	} {
		if !contains(got, want) {
			t.Fatalf("verifier prompt is missing %q:\n%s", want, got)
		}
	}
	i, j := strings.Index(fold(got), "## The pull request to verify"), strings.Index(fold(got), "## When you are done")
	if i < 0 || j < 0 || i > j {
		t.Fatalf("the verifier's required section must sit after the PR is named:\n%s", got)
	}
	if k := strings.Index(fold(got), "execute it, then summarise"); k < j {
		t.Fatalf("the ordering statement must sit inside the required section:\n%s", got)
	}
}

// TASK-65: the verifier is told where its run stands — a worktree
// provisioned at the PR's head — so it never fetches, clones or cuts one of
// its own before judging, and never touches the project's checkout instead.
func TestTheVerifierIsToldItsRunIsProvisionedAtThePRHead(t *testing.T) {
	got := verify(fleet.Agent{Name: "rev", Persona: "R"},
		work.Task{ID: "TASK-3", Title: "T"}, "https://github.com/o/r/pull/7", "", "/fleet")
	for _, want := range []string{
		"own worktree",
		"checked out at that pull request's head commit",
		"no fetching and no checkout to do first",
		"removed when you settle",
	} {
		if !contains(got, want) {
			t.Errorf("verifier prompt lacks %q:\n%s", want, got)
		}
	}
}

// In a queue with a verifier, a worker's PR is a delivery, not a close: the
// prompt says the task stays open and never teaches the agent to merge or to
// hand the task on itself.
func TestAWorkerInAVerifiedQueueDeliversItsPRAndNeverHandsOn(t *testing.T) {
	got := deliver(fleet.Agent{Name: "dev", Persona: "P"}, work.Task{ID: "TASK-3", Title: "T"}, "", "", "/fleet", fleet.Words{})
	for _, want := range []string{
		"herdr-docket task done TASK-3 --pr",
		"stays open",
		"verifier",
		"Never merge",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("worker prompt lacks %q", want)
		}
	}
	if strings.Contains(got, "task assign") {
		t.Error("a verified queue's worker must not be taught to hand the task on")
	}
}

// The verifier is told the PR, the one command that records its verdict, and
// that it neither merges nor closes.
func TestTheVerifierIsToldThePRAndTheVerdictCommandOnly(t *testing.T) {
	got := verify(fleet.Agent{Name: "rev", Persona: "R"}, work.Task{ID: "TASK-3", Title: "T", Body: "verify by running X"}, "https://github.com/o/r/pull/7", "", "/fleet")
	for _, want := range []string{
		"R",
		"verify by running X",
		"https://github.com/o/r/pull/7",
		"herdr-docket task verdict TASK-3 PASS --pr \"https://github.com/o/r/pull/7\"",
		"herdr-docket task verdict TASK-3 FAIL --pr \"https://github.com/o/r/pull/7\"",
		"never merge",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("verifier prompt lacks %q", want)
		}
	}
	for _, unwanted := range []string{"task done", "task assign", "task fail", "task block"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("verifier prompt teaches %q — the runner closes the task, not the verifier", unwanted)
		}
	}
}
