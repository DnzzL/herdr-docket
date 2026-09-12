package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/fleet"
	"github.com/DnzzL/herdr-fleet/internal/work"
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
		"/Users/x/fleet",
	)
	for _, want := range []string{
		"You run publication.",
		"TASK-7", "Draft launch post", "Write the Show HN post.",
		"[ ] #1 under 200 words", "[x] #2 links the repo",
		"previous attempt stalled",
		"herdr-fleet task note TASK-7",
		"herdr-fleet task done TASK-7",
		"herdr-fleet task fail TASK-7",
		"herdr-fleet task block TASK-7",
		"herdr-fleet task create",
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
		work.Task{ID: "T-1", Title: "t", Open: true, Body: "b"}, "/d")
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
			Criteria: []work.Criterion{{Index: 1, Text: "works"}}}, "/d")
	if !strings.Contains(got, "not yours to edit") {
		t.Fatalf("prompt does not say the criteria are read-only:\n%s", got)
	}
	if !strings.Contains(got, "which criteria you met") {
		t.Fatalf("prompt does not ask for the verdict in the closing note:\n%s", got)
	}
}

func TestAssembleOmitsEmptySections(t *testing.T) {
	got := Assemble(fleet.Agent{Name: "a", Persona: "P"},
		work.Task{ID: "T-1", Title: "t", Open: true}, "/d")
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
		work.Task{ID: "myapp/TASK-12", Title: "t", Open: true}, "/d")
	if !strings.Contains(got, "task create \"<title>\" -d \"<what and why>\" -a a -s myapp") {
		t.Fatalf("follow-up create must carry -s myapp:\n%s", got)
	}
}

// A single queue has no prefix and no source to name: the follow-up command
// must stay exactly as it was, with no -s on it.
func TestAssembleLeavesTheFollowUpAloneForAQueueWithNoPrefix(t *testing.T) {
	got := Assemble(fleet.Agent{Name: "a", Persona: "P"},
		work.Task{ID: "TASK-12", Title: "t", Open: true}, "/d")
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
		dir,
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
	got := Assemble(a, v, dir)
	if want := assemble(a, v, "", dir); got != want {
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
	if got, want := Assemble(a, v, dir), assemble(a, v, "", dir); got != want {
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
	if got, want := Assemble(a, v, dir), assemble(a, v, "", dir); got != want {
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
		work.Task{ID: "TASK-1", Title: "t", Open: true}, dir)
	if !strings.Contains(got, "We build acme.") || !strings.Contains(got, "Keep this note.") {
		t.Fatalf("a brief with prose must be prepended whole:\n%s", got)
	}
}
