package prompt

import (
	"strings"
	"testing"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/work"
)

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
