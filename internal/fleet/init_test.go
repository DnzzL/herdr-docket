package fleet

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
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

// The triager's role is the grill's whole method (TASK-86): when to ask,
// how a round reads, the confirm round that alone publishes, and the label
// that forces a round.
func TestTheTriagerRoleCarriesTheGrill(t *testing.T) {
	dir := t.TempDir()
	initFleet(t, dir)
	role := read(t, filepath.Join(dir, "roles", "triager.md"))
	for _, want := range []string{
		"ASSUMPTIONS:",                                 // the clear/uncertain rule
		"## Round 2\n", "(reco)", "T. try and show me", // the Round format
		"lines of context",            // what the owner reads before question 1
		"Build on these assumptions?", // the final confirm round
		"`grill`",                     // the label that forces a round
		"task ask <id>", "docs/agents/issue-tracker.md",
	} {
		if !strings.Contains(role, want) {
			t.Errorf("roles/triager.md lacks %q", want)
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

// init <repo> declares the grill's statuses too: the CLI refuses a status the
// config does not list, so a repo without them could never hold an idea.
func TestInitRepoAddsTheTriageAndQuestionStatuses(t *testing.T) {
	s, repo := repoWithBacklog(t)
	if err := InitRepo(s, repo, io.Discard); err != nil {
		t.Fatal(err)
	}
	config := read(t, filepath.Join(repo, "backlog", "config.yml"))
	for _, status := range []string{"Triage", "Question"} {
		if !strings.Contains(config, `"`+status+`"`) {
			t.Errorf("config.yml lacks %q:\n%s", status, config)
		}
	}
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

// fleetPath is fleet.yaml in the test's config dir.
func fleetPath() string {
	return filepath.Join(os.Getenv("HERDR_PLUGIN_CONFIG_DIR"), "fleet.yaml")
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// githubRepoDir is a git repo whose origin is origin.
func githubRepoDir(t *testing.T, origin string) string {
	t.Helper()
	repo := t.TempDir()
	args := [][]string{{"init", "-q"}}
	if origin != "" {
		args = append(args, []string{"remote", "add", "origin", origin})
	}
	for _, a := range args {
		if out, err := command(repo, "git", a...); err != nil {
			t.Fatalf("git %v: %s", a, out)
		}
	}
	return repo
}

// A fleet.yaml from before ADR 0023 names a Backlog.md source with no dir: the
// queue used to live in the fleet dir. init <repo> is the migration: it loads,
// and names the repo on that very source, keeping its other fields and the
// owner's comments.
func TestInitRepoMigratesALegacySourceWithNoDir(t *testing.T) {
	s, repo := repoWithBacklog(t)
	writeFile(t, fleetPath(), "dir: "+s.Dir+"\n# our queue\nsource:\n  kind: backlogmd\n  verifier: rev # judges PRs\n")
	if err := InitRepo(loaded(t), repo, io.Discard); err != nil {
		t.Fatal(err)
	}
	got := loaded(t)
	if got.Source.Dir != repo || got.Source.Verifier != "rev" || len(got.Sources) != 0 {
		t.Fatalf("source = %+v, sources = %v; want the legacy source with dir %s", got.Source, got.Sources, repo)
	}
	raw := read(t, fleetPath())
	for _, c := range []string{"# our queue", "# judges PRs"} {
		if !strings.Contains(raw, c) {
			t.Errorf("comment %q lost:\n%s", c, raw)
		}
	}
}

// A fleet.yaml that already names another queue gets this repo beside it:
// source: becomes sources:, each named by its repo, the old one stays where
// new tasks land, and the owner's comments survive.
func TestInitRepoAddsTheRepoBesideAnotherQueue(t *testing.T) {
	s, repo := repoWithBacklog(t)
	writeFile(t, fleetPath(), "dir: "+s.Dir+"\n# my fleet\nsource:\n  kind: github # the board\n  github:\n    owner: acme\n    project: 3\n    repo: acme/app\n")
	if err := InitRepo(loaded(t), repo, io.Discard); err != nil {
		t.Fatal(err)
	}
	got := loaded(t)
	if !reflect.DeepEqual(got.Source, SourceConfig{}) || len(got.Sources) != 2 {
		t.Fatalf("want two sources, got source %+v sources %+v", got.Source, got.Sources)
	}
	if c := got.Sources["app"]; c.Kind != kindGithub || c.Github.Repo != "acme/app" {
		t.Errorf("the old queue is not sources.app: %+v", got.Sources)
	}
	if c := got.Sources[filepath.Base(repo)]; c.Dir != repo {
		t.Errorf("the repo is not sources.%s: %+v", filepath.Base(repo), got.Sources)
	}
	if got.DefaultSource != "app" {
		t.Errorf("default_source = %q, want the queue new tasks already landed in", got.DefaultSource)
	}
	raw := read(t, fleetPath())
	for _, c := range []string{"# my fleet", "# the board"} {
		if !strings.Contains(raw, c) {
			t.Errorf("comment %q lost:\n%s", c, raw)
		}
	}
	if _, err := NewSource(got); err != nil {
		t.Errorf("the fleet does not build its queue: %v", err)
	}
	before := read(t, fleetPath())
	if err := InitRepo(loaded(t), repo, io.Discard); err != nil {
		t.Fatal(err)
	}
	if after := read(t, fleetPath()); after != before {
		t.Errorf("a second run changed fleet.yaml:\n%s\n---\n%s", before, after)
	}
}

// sources: gets one more entry; a name already taken gets a suffix rather than
// overwriting somebody's queue. A dir YAML would misread unquoted (# and : )
// round-trips.
func TestInitRepoAddsToSourcesUnderAFreeName(t *testing.T) {
	s, _ := repoWithBacklog(t)
	repo := filepath.Join(t.TempDir(), "a #b: c")
	if err := os.MkdirAll(filepath.Join(repo, "backlog"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "backlog", "config.yml"), "statuses: [\"To Do\"]\n")
	name := filepath.Base(repo)
	writeFile(t, fleetPath(), "dir: "+s.Dir+"\nsources:\n  \""+name+"\":\n    dir: /srv/other\n")
	if err := InitRepo(loaded(t), repo, io.Discard); err != nil {
		t.Fatal(err)
	}
	got := loaded(t).Sources
	if got[name].Dir != "/srv/other" || got[name+"-2"].Dir != repo {
		t.Fatalf("sources = %+v, want %q kept and %q-2 at %s", got, name, name, repo)
	}
}

// The --github path adds beside an existing queue too.
func TestInitGithubRepoAddsTheRepoBesideAnotherQueue(t *testing.T) {
	s, other := repoWithBacklog(t)
	writeFile(t, fleetPath(), "dir: "+s.Dir+"\nsource:\n  dir: "+other+"\n")
	repo := githubRepoDir(t, "https://github.com/org/app.git")
	if err := InitGithubRepo(loaded(t), repo, "org/3", io.Discard); err != nil {
		t.Fatal(err)
	}
	got := loaded(t).Sources
	if got[filepath.Base(other)].Dir != other || got[filepath.Base(repo)].Github.Repo != "org/app" {
		t.Fatalf("sources = %+v, want the old repo and org/app", got)
	}
}

// Every way init <repo> refuses says what is wrong.
func TestInitRepoRefusals(t *testing.T) {
	s, _ := repoWithBacklog(t)
	gitlab := githubRepoDir(t, "git@gitlab.com:org/app.git")
	noOrigin := githubRepoDir(t, "")
	github := githubRepoDir(t, "git@github.com:org/app.git")
	for _, c := range []struct {
		name, repo, board, want string
	}{
		{"not a directory", filepath.Join(gitlab, "nope"), "", "is not a directory"},
		{"not a directory, github", filepath.Join(gitlab, "nope"), "org/3", "is not a directory"},
		{"board with no number", github, "org", "<owner>/<project-number>"},
		{"board with a word for a number", github, "org/x", "<owner>/<project-number>"},
		{"board with no owner", github, "/3", "<owner>/<project-number>"},
		{"board zero", github, "org/0", "<owner>/<project-number>"},
		{"origin elsewhere", gitlab, "org/3", "is not a GitHub repository"},
		{"no origin", noOrigin, "org/3", "no origin remote"},
	} {
		var err error
		if c.board == "" {
			err = InitRepo(s, c.repo, io.Discard)
		} else {
			err = InitGithubRepo(s, c.repo, c.board, io.Discard)
		}
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
	}
}

// A triage run publishes a card that already exists: the tracker docs it
// follows must say to edit that card's body, not create a second spec, and to
// set it To Do with the tracker's own command. The repo's own copy of the
// Backlog.md doc is the shipped one, byte for byte.
func TestTheTrackerDocsSayATriageRunEditsTheCardItHas(t *testing.T) {
	for path, wants := range map[string][]string{
		"issue-tracker.md":        {"already exists", `backlog task edit <id> -d "<spec body>" -s "To Do" -a <agent>`},
		"issue-tracker-github.md": {"already exists", "gh issue edit <n> -R {{repo}} --body-file", "--field Status --value Todo", "--field Agent --value <agent>"},
	} {
		doc := read(t, path)
		for _, want := range wants {
			if !strings.Contains(doc, want) {
				t.Errorf("%s lacks %q", path, want)
			}
		}
	}
	if read(t, "issue-tracker.md") != read(t, "../../docs/agents/issue-tracker.md") {
		t.Error("docs/agents/issue-tracker.md has drifted from the shipped issue-tracker.md")
	}
}
