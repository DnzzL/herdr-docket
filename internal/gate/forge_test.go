package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gh is exec'd, not injected, so the seam these tests hold is PATH: the
// script answers the way gh answers and records every argv the adapter sent,
// the same boundary pin the herdr client keeps for its own invocations. A
// wrong flag here would only ever be seen in someone's failed hold.
const prURL = "https://github.com/o/r/pull/7"

// fakeGH installs a gh on PATH that logs each invocation to argsFile and
// answers `pr view` with viewJSON (empty prints {}), exiting with createExit
// for `label create` and 0 otherwise.
func fakeGH(t *testing.T, argsFile, viewJSON, createExit string) {
	t.Helper()
	script := `#!/bin/sh
printf '%s\n' "$*" >> ` + argsFile + `
if [ "$1" = "label" ] && [ "$2" = "create" ]; then exit ` + createExit + `; fi
if [ "$1" = "pr" ] && [ "$2" = "view" ]; then printf '%s' '` + viewJSON + `'; fi
exit 0
`
	dir := t.TempDir()
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func calls(t *testing.T, argsFile string) []string {
	t.Helper()
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// The label is created in the repo only if missing; applying it to the PR is
// the step that must happen either way, so a create that fails because the
// label already exists is not a failed marking.
func TestAddLabelCreatesTheLabelInRepoThenAppliesItToThePR(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	fakeGH(t, argsFile, "", "0")
	if err := (GH{}).AddLabel(prURL, "merge-needed"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}
	got := calls(t, argsFile)
	want := []string{
		"label create merge-needed --repo o/r",
		"pr edit " + prURL + " --add-label merge-needed",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("gh argv:\n%q\nwant:\n%q", got, want)
	}
}

func TestAddLabelStillAppliesWhenTheLabelAlreadyExists(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	fakeGH(t, argsFile, "", "1") // gh refuses `label create` on an existing label
	if err := (GH{}).AddLabel(prURL, "merge-needed"); err != nil {
		t.Fatalf("AddLabel = %v, want nil: the label existing is the outcome wanted", err)
	}
	got := calls(t, argsFile)
	if len(got) != 2 || !strings.Contains(got[1], "--add-label merge-needed") {
		t.Fatalf("gh argv = %q, want the create attempt then the application", got)
	}
}

// Removal reads first: a repo that never marked anything does not have the
// label, and gh refuses to remove a name it has never heard of — a refusal
// that would log on every clean merge for a state already correct.
func TestRemoveLabelTakesItOffOnlyWhenThePRCarriesIt(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	fakeGH(t, argsFile, `{"labels":[{"name":"merge-needed"}]}`, "0")
	if err := (GH{}).RemoveLabel(prURL, "merge-needed"); err != nil {
		t.Fatalf("RemoveLabel: %v", err)
	}
	got := calls(t, argsFile)
	want := []string{
		"pr view " + prURL + " --json labels",
		"pr edit " + prURL + " --remove-label merge-needed",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("gh argv:\n%q\nwant:\n%q", got, want)
	}
}

func TestRemoveLabelIsSilentWhenThereIsNothingToRemove(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	fakeGH(t, argsFile, `{"labels":[]}`, "0")
	if err := (GH{}).RemoveLabel(prURL, "merge-needed"); err != nil {
		t.Fatalf("RemoveLabel = %v, want nil", err)
	}
	if got := calls(t, argsFile); len(got) != 1 {
		t.Fatalf("gh argv = %q, want only the read — nothing to remove is not an error", got)
	}
}

// The popup a hold raises names the PR the way a human and the gh CLI both
// do; anything else is not a pull-request URL and must not be guessed at.
func TestRepoRefSplitsAPullRequestURL(t *testing.T) {
	repo, number, err := RepoRef(prURL)
	if err != nil || repo != "o/r" || number != "7" {
		t.Fatalf("RepoRef = %q, %q, %v — want o/r, 7, nil", repo, number, err)
	}
	for _, bad := range []string{"https://github.com/o/r", "https://github.com/o/r/issues/7", "not a url"} {
		if _, _, err := RepoRef(bad); err == nil {
			t.Errorf("RepoRef(%q) accepted what is not a pull request", bad)
		}
	}
}

// cannedGH is a `gh` on the PATH that records every invocation and answers by
// matching an argument against a canned reply table — the port is the CLI,
// so the test stands in for it. {"api repos/o/r/...": "body"} print body;
// {"*": "@exit"]} makes gh die with 1, `gh: Not Found` etc. as stderr.
type cannedGH struct {
	dir   string
	log   string
	match []canned
}

type canned struct {
	find   string
	stdout string
	stderr string
	exit   int
}

func (f *cannedGH) install(t *testing.T) {
	t.Helper()
	f.dir = t.TempDir()
	f.log = filepath.Join(f.dir, "invocations")
	f.write(t)
	t.Setenv("PATH", f.dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func (f *cannedGH) write(t *testing.T) {
	t.Helper()
	body := "#!/bin/sh\n" +
		"echo \"$@\" >> \"" + f.log + "\"\n"
	body += "case \"$*\" in\n"
	for _, c := range f.match {
		target := 2
		if c.stderr == "" {
			target = 1
		}
		body += fmt.Sprintf("  *%q*) printf '%%s\\n' %q >&%d; exit %d ;;\n",
			c.find, c.stdout+c.stderr, target, c.exit)
	}
	body += "esac\n"
	if err := os.WriteFile(filepath.Join(f.dir, "gh"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func (f *cannedGH) invocations(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(f.log)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

// The base ref is asked of the PR, the file of the API with that ref, raw:
// the exact pair of calls the gate's runs make (TASK-57).
func TestGHCodeownersFollowsThePrsBaseRef(t *testing.T) {
	f := &cannedGH{match: []canned{
		{"pr view", `{"baseRefName":"main"}`, "", 0},
		{"contents/.github/CODEOWNERS", "/db/ @someone", "", 0},
	}}
	f.install(t)
	got, err := GH{}.Codeowners("https://github.com/o/r/pull/7")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "/db/" {
		t.Fatalf("patterns = %q, want /db/", got)
	}
	seen := strings.Join(f.invocations(t), "\n")
	for _, want := range []string{
		"pr view https://github.com/o/r/pull/7 --json baseRefName",
		"api repos/o/r/contents/.github/CODEOWNERS?ref=main -H Accept: application/vnd.github.raw",
	} {
		if !strings.Contains(seen, want) {
			t.Fatalf("args must name %q, saw:\n%s", want, seen)
		}
	}
}

// A file at the last of GitHub's three places is found after two misses,
// each asked in GitHub's own order.
func TestGHCodeownersTriesTheThreePlacesGitHubLooks(t *testing.T) {
	f := &cannedGH{match: []canned{
		{"pr view", `{"baseRefName":"main"}`, "", 0},
		{"contents/docs/CODEOWNERS", "docs/ @someone", "", 0},
		{"contents/", "", "gh: Not Found (HTTP 404)", 1}, // the other two miss
	}}
	f.install(t)
	got, err := GH{}.Codeowners("https://github.com/o/r/pull/7")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "docs/" {
		t.Fatalf("patterns = %q, want docs/", got)
	}
	for _, path := range codeownerPaths {
		if !strings.Contains(strings.Join(f.invocations(t), "\n"), "contents/"+path) {
			t.Fatalf("%s was never tried:\n%v", path, f.invocations(t))
		}
	}
}

// No file anywhere GitHub looks is no protected path — nil and no error, so
// the gate decides exactly as it did for a checkout without one (TASK-57).
func TestGHCodeownersNoFileOnTheBaseIsNoProtectedPath(t *testing.T) {
	f := &cannedGH{match: []canned{
		{"pr view", `{"baseRefName":"main"}`, "", 0},
		{"contents/", "", "gh: Not Found (HTTP 404)", 1},
	}}
	f.install(t)
	got, err := GH{}.Codeowners("https://github.com/o/r/pull/7")
	if got != nil || err != nil {
		t.Fatalf("got %q err %v, want nil and no error", got, err)
	}
}

// A forge that cannot answer is not a silent absence: the error travels so
// the task is held rather than merged unguarded.
func TestGHCodeownersSurfacesForgeErrors(t *testing.T) {
	f := &cannedGH{match: []canned{
		{"pr view", `{"baseRefName":"main"}`, "", 0},
		{"contents/", "", "gh: Bad Gateway (HTTP 502)", 1},
	}}
	f.install(t)
	_, err := GH{}.Codeowners("https://github.com/o/r/pull/7")
	if err == nil {
		t.Fatal("want the forge's failure to travel, not nil")
	}
}

// Only gh's own `HTTP 404` line means no file: a 401 whose endpoint or
// stderr mentions a repo/branch with 404 in the name must surface as an
// error, not be read as the file is absent (the endpoint carries the
// owner/repo/ref, so matching the whole message would fail open).
func TestGHCodeownersOnlyReal404sMeanNoFile(t *testing.T) {
	f := &cannedGH{match: []canned{
		{"pr view", `{"baseRefName":"main"}`, "", 0},
		{"contents/", "", "gh: Bad credentials (HTTP 401)", 1},
	}}
	f.install(t)
	_, err := GH{}.Codeowners("https://github.com/acme/my-404-repo/pull/7")
	if err == nil {
		t.Fatal("a 401 with 404 in the endpoint is not a missing file: want the error to travel")
	}
}

// Only the whole https://github.com/owner/repo/pull/N form parses — the
// runner only carries that form, and a half-match would fetch another
// repo's rules.
func TestGithubRepoReadsOwnerAndRepoFromThePullUrl(t *testing.T) {
	owner, repo, err := githubRepo("https://github.com/DnzzL/herdr-docket/pull/57")
	if err != nil || owner != "DnzzL" || repo != "herdr-docket" {
		t.Fatalf("owner=%q repo=%q err=%v", owner, repo, err)
	}
	for _, bad := range []string{"https://github.com/o/r/issue/7", "https://x.dev/o/r/pull/7", "o/r#7"} {
		if _, _, err := githubRepo(bad); err == nil {
			t.Errorf("%q parsed as a pull request url, must not", bad)
		}
	}
}
