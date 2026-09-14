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
		"herdr-docket task note TASK-7",
		"herdr-docket task done TASK-7",
		"herdr-docket task fail TASK-7",
		"herdr-docket task block TASK-7",
		"herdr-docket task create",
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
	if !strings.Contains(got, "task create \"<title>\" -d \"<what and why>\" -a a -s myapp") {
		t.Fatalf("follow-up create must carry -s myapp:\n%s", got)
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
