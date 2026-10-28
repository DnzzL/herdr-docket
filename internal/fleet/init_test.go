package fleet

import (
	"bytes"
	"io"
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

// init <repo> works on a repo whose board humans already use: the fleet's
// statuses are added, the repo's own columns are kept.
func TestPatchStatusesKeepsTheRepoOwnStatuses(t *testing.T) {
	path := configWith(t, "statuses: [\"Draft\", \"To Do\", \"Review\", \"Done\"]\n")
	if err := patchStatuses(path); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	for _, status := range append([]string{"Draft", "Review"}, backlogmd.Statuses...) {
		if !strings.Contains(got, `"`+status+`"`) {
			t.Errorf("config lost or lacks %q:\n%s", status, got)
		}
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
	for _, name := range []string{"dev", "reviewer", "example"} {
		if got := read(t, filepath.Join(dir, "roles", name+".md")); got == "" {
			t.Errorf("roles/%s.md is empty", name)
		}
	}
	for _, name := range []string{"dev", "reviewer"} {
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
	for _, name := range []string{"dev", "reviewer"} {
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
	for _, name := range []string{"dev", "reviewer"} {
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

// repoWithBacklog is a project repo whose Backlog.md project already exists,
// so init <repo> needs no Backlog.md CLI, and a config dir with no fleet.yaml.
func repoWithBacklog(t *testing.T) (Settings, string) {
	t.Helper()
	config, fleetDir, repo := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", config)
	if err := os.WriteFile(filepath.Join(config, "fleet.yaml"), []byte("dir: "+fleetDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "backlog"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "backlog", "config.yml"), []byte("project_name: acme\nstatuses: [\"To Do\", \"Done\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return loaded(t), repo
}

// loaded is fleet.yaml as the CLI reads it on each run.
func loaded(t *testing.T) Settings {
	t.Helper()
	s, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// init <repo> makes a repo ready for the factory in one command: the fleet's
// statuses in its Backlog.md config, the tracker doc the skills read, and the
// repo named as the fleet's queue.
func TestInitRepoPreparesTheRepoAndNamesItTheQueue(t *testing.T) {
	s, repo := repoWithBacklog(t)
	if err := InitRepo(s, repo, io.Discard); err != nil {
		t.Fatal(err)
	}
	config := read(t, filepath.Join(repo, "backlog", "config.yml"))
	for _, status := range backlogmd.Statuses {
		if !strings.Contains(config, `"`+status+`"`) {
			t.Errorf("config.yml lacks %q:\n%s", status, config)
		}
	}
	doc := read(t, filepath.Join(repo, "docs", "agents", "issue-tracker.md"))
	for _, want := range []string{"backlog task create", "--parent", "backlog task", "--plain", "--check-ac"} {
		if !strings.Contains(doc, want) {
			t.Errorf("issue-tracker.md does not mention %q:\n%s", want, doc)
		}
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "FLEET.md")); err != nil {
		t.Errorf("init <repo> must create the fleet too: %v", err)
	}
	if got := loaded(t).Source.Dir; got != repo {
		t.Errorf("fleet.yaml source.dir = %q, want %q", got, repo)
	}
}

// Running it twice changes nothing, and a tracker doc the repo already has is
// its owner's: kept, and said so.
func TestInitRepoTwiceIsANoOpAndKeepsAnExistingTrackerDoc(t *testing.T) {
	s, repo := repoWithBacklog(t)
	doc := filepath.Join(repo, "docs", "agents", "issue-tracker.md")
	if err := os.MkdirAll(filepath.Dir(doc), 0o755); err != nil {
		t.Fatal(err)
	}
	mine := "# Issue tracker: ours\n"
	if err := os.WriteFile(doc, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InitRepo(s, repo, io.Discard); err != nil {
		t.Fatal(err)
	}
	files := []string{
		filepath.Join(repo, "backlog", "config.yml"),
		doc,
		filepath.Join(os.Getenv("HERDR_PLUGIN_CONFIG_DIR"), "fleet.yaml"),
	}
	before := map[string]string{}
	for _, f := range files {
		before[f] = read(t, f)
	}
	var out bytes.Buffer
	if err := InitRepo(loaded(t), repo, &out); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if got := read(t, f); got != before[f] {
			t.Errorf("second run changed %s:\n%s\n---\n%s", f, before[f], got)
		}
	}
	if got := read(t, doc); got != mine {
		t.Errorf("an existing issue-tracker.md was overwritten:\n%s", got)
	}
	if !strings.Contains(out.String(), "kept docs/agents/issue-tracker.md") {
		t.Errorf("init must say it kept the tracker doc:\n%s", out.String())
	}
}

// A fleet.yaml that already names another queue is the owner's YAML: init
// never rewrites it, it prints the block to paste and leaves the file as is.
func TestInitRepoNeverRewritesAFleetYAMLThatNamesAnotherQueue(t *testing.T) {
	s, repo := repoWithBacklog(t)
	path := filepath.Join(os.Getenv("HERDR_PLUGIN_CONFIG_DIR"), "fleet.yaml")
	mine := "dir: " + s.Dir + "\n# my fleet\nsource:\n  kind: github\n  github:\n    owner: acme\n    project: 3\n"
	if err := os.WriteFile(path, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := InitRepo(loaded(t), repo, &out); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != mine {
		t.Errorf("fleet.yaml was rewritten:\n%s", got)
	}
	if !strings.Contains(out.String(), "dir: "+repo) {
		t.Errorf("init must print the block to paste:\n%s", out.String())
	}
}

// init <repo> --github org/3 prepares a GitHub-tracked repo: the GitHub
// tracker doc, a github source naming the board and the repo read from its
// origin, and no Backlog.md project.
func TestInitGithubRepoWritesTheGithubTemplateAndSource(t *testing.T) {
	config, fleetDir, repo := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", config)
	if err := os.WriteFile(filepath.Join(config, "fleet.yaml"), []byte("dir: "+fleetDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "git@github.com:org/app.git"}} {
		if out, err := command(repo, "git", args...); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	if err := InitGithubRepo(loaded(t), repo, "org/3", io.Discard); err != nil {
		t.Fatal(err)
	}
	doc := read(t, filepath.Join(repo, "docs", "agents", "issue-tracker.md"))
	for _, want := range []string{"gh issue create", "--parent", "gh project item-add 3 --owner org", "Blocked by: #", "gh issue view", "gh issue close"} {
		if !strings.Contains(doc, want) {
			t.Errorf("issue-tracker.md does not mention %q:\n%s", want, doc)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "backlog")); !os.IsNotExist(err) {
		t.Errorf("a GitHub repo must get no backlog/: %v", err)
	}
	got := loaded(t).Source
	if got.Kind != "github" || got.Github.Owner != "org" || got.Github.Project != 3 || got.Github.Repo != "org/app" {
		t.Errorf("fleet.yaml source = %+v, want kind github, owner org, project 3, repo org/app", got)
	}
	var out bytes.Buffer
	before := read(t, filepath.Join(config, "fleet.yaml"))
	if err := InitGithubRepo(loaded(t), repo, "org/3", &out); err != nil {
		t.Fatal(err)
	}
	if after := read(t, filepath.Join(config, "fleet.yaml")); after != before {
		t.Errorf("second run changed fleet.yaml:\n%s\n---\n%s", before, after)
	}
}

func TestGithubRepoFromOrigin(t *testing.T) {
	for _, url := range []string{"git@github.com:org/app.git", "https://github.com/org/app", "https://github.com/org/app.git", "ssh://git@github.com/org/app.git"} {
		if got, err := githubRepo(url); err != nil || got != "org/app" {
			t.Errorf("githubRepo(%q) = %q, %v; want org/app", url, got, err)
		}
	}
	if _, err := githubRepo("git@gitlab.com:org/app.git"); err == nil {
		t.Error("a non-GitHub origin must be refused")
	}
}
