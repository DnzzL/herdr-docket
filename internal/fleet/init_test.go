package fleet

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DnzzL/herdr-docket/internal/work/backlogmd"
)

// The Backlog.md CLI refuses any status the project config does not declare,
// so a status the adapter can write and the config does not list is a run that
// dies at the first phase change. The two lists are one list.
func TestPatchStatusesWritesEveryStatusTheAdapterUses(t *testing.T) {
	path := configWith(t, "project_name: fleet\nstatuses: [\"To Do\", \"Done\"]\n")
	if err := patchStatuses(path); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	for _, status := range backlogmd.Statuses {
		if !strings.Contains(got, `"`+status+`"`) {
			t.Errorf("config is missing %q the adapter can write:\n%s", status, got)
		}
	}
	for _, gone := range []string{`statuses: ["To Do", "Done"]`} {
		if strings.Contains(got, gone) {
			t.Errorf("the old line survived:\n%s", got)
		}
	}
	if !strings.Contains(got, "project_name: fleet") {
		t.Errorf("patching ate the rest of the config:\n%s", got)
	}
}

// Re-running init is normal — an existing fleet is left alone.
func TestPatchStatusesIsIdempotent(t *testing.T) {
	path := configWith(t, "statuses: [\"To Do\"]\n")
	if err := patchStatuses(path); err != nil {
		t.Fatal(err)
	}
	once := read(t, path)
	if err := patchStatuses(path); err != nil {
		t.Fatal(err)
	}
	if twice := read(t, path); twice != once {
		t.Fatalf("second pass changed the file:\n%s\n%s", once, twice)
	}
}

// A config we cannot patch is a config we say so about, rather than one we
// silently leave unable to run anything.
func TestPatchStatusesRefusesAConfigItCannotPatch(t *testing.T) {
	path := configWith(t, "project_name: fleet\n")
	if err := patchStatuses(path); err == nil {
		t.Fatal("want an error when there is no statuses line to patch")
	}
}

// init writes a commented FLEET.md the first time it runs, so the shared
// brief's existence is discoverable. Every line is a comment: an untouched
// scaffold must never reach an agent.
func TestInitBriefScaffoldsACommentedExample(t *testing.T) {
	dir := t.TempDir()
	if err := initBrief(dir); err != nil {
		t.Fatal(err)
	}
	got := read(t, filepath.Join(dir, "FLEET.md"))
	if !strings.Contains(got, "FLEET.md") {
		t.Fatalf("the scaffold does not say what the file is:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			t.Fatalf("scaffold line is not a comment: %q", line)
		}
	}
}

// A human's brief is not init's to overwrite: re-running init is normal and
// must leave what is already there alone.
func TestInitBriefLeavesAnExistingBriefAlone(t *testing.T) {
	dir := t.TempDir()
	mine := "We are Acme.\n"
	path := filepath.Join(dir, "FLEET.md")
	if err := os.WriteFile(path, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := initBrief(dir); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != mine {
		t.Fatalf("init changed an existing brief:\n%q", got)
	}
}

func configWith(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// initFleet runs init the way the CLI does, over a queue that lives elsewhere
// so the test needs no Backlog.md CLI, and returns what init printed.
func initFleet(t *testing.T, dir string) string {
	t.Helper()
	var out bytes.Buffer
	if err := Init(Settings{Dir: dir, Source: SourceConfig{Kind: kindGithub}}, &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// The pipeline's two roles ship with plain init (ADR 0021): a fleet has the
// author's and the verifier's method from its first run, beside the example,
// without installing any loop — and init says what it wrote.
func TestInitWritesTheTwoPipelineRoles(t *testing.T) {
	dir := t.TempDir()
	out := initFleet(t, dir)
	for _, name := range []string{"dev", "reviewer", "triager", "example"} {
		if got := read(t, filepath.Join(dir, "roles", name+".md")); got == "" {
			t.Errorf("roles/%s.md is empty", name)
		}
	}
	for _, name := range []string{"dev", "reviewer", "triager"} {
		if !strings.Contains(out, "wrote roles/"+name+".md") {
			t.Errorf("output must say it wrote roles/%s.md:\n%s", name, out)
		}
	}
}

// The manual and the shipped bytes must not drift: docs/examples.md shows the
// roles a fleet copies by hand, and these are the ones init writes. Read
// against the docs on disk — if someone edits one side, this goes red until
// both agree.
func TestTheRolesInitWritesAreTheOnesTheDocsShow(t *testing.T) {
	examples := read(t, "../../docs/examples.md")
	dir := t.TempDir()
	initFleet(t, dir)
	for _, name := range []string{"dev", "reviewer", "triager"} {
		if !strings.Contains(examples, read(t, filepath.Join(dir, "roles", name+".md"))) {
			t.Errorf("role %s is not in docs/examples.md verbatim — one of the two has drifted", name)
		}
	}
}

// A role the fleet already has is its owner's method: re-running init keeps
// it byte for byte and says it was skipped.
func TestInitKeepsARoleTheFleetAlreadyHas(t *testing.T) {
	dir := t.TempDir()
	initFleet(t, dir)
	mine := filepath.Join(dir, "roles", "dev.md")
	if err := os.WriteFile(mine, []byte("my own method"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := initFleet(t, dir)
	if got := read(t, mine); got != "my own method" {
		t.Fatalf("roles/dev.md overwritten: %q", got)
	}
	for _, name := range []string{"dev", "reviewer", "triager"} {
		if !strings.Contains(out, "skipped roles/"+name+".md") {
			t.Errorf("output must say it skipped roles/%s.md:\n%s", name, out)
		}
	}
}

// verdictCommand matches a `task verdict` command example, tolerant of the
// line wraps markdown puts inside a backticked command.
var verdictCommand = regexp.MustCompile("task verdict[^`\\n]*(?:\\n[^`]*)?")

// The instruction to carry the evidence is the --note flag: without it an
// agent following the docs verbatim records a bare verdict, and the evidence
// it gathered is stranded in the task notes instead of on the PR comment.
// Every command example in the docs and the shipped roles must carry it —
// this omission already produced a bare verdict once (docket TASK-71).
func TestEveryVerdictExampleCarriesItsNote(t *testing.T) {
	for _, path := range []string{
		"../../docs/commands.md",
		"../../docs/examples.md",
		"../../docs/adr/0013-the-factory-is-an-author-a-verifier-and-a-gate-in-code.md",
		"roles/reviewer.md",
	} {
		for _, example := range verdictCommand.FindAllString(read(t, path), -1) {
			if !strings.Contains(example, "--note") {
				t.Errorf("%s: a task verdict example without --note teaches a bare verdict: %q", path, example)
			}
		}
	}
}

// The shipped triager must say the four things a triage run cannot invent:
// when an idea is clear, what a round looks like, that the last round is the
// confirmation, and that the grill label forces the questions (TASK-86).
func TestTheShippedTriagerNamesTheGrillRules(t *testing.T) {
	role := read(t, "roles/triager.md")
	for _, want := range []string{
		"clear", "uncertain", // the rule
		"## Round 2",                  // the round format
		"Build on these assumptions?", // the final confirm round
		"grill",                       // the label that forces the questions
	} {
		if !strings.Contains(role, want) {
			t.Errorf("roles/triager.md does not name %q", want)
		}
	}
}
