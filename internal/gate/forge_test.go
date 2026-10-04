package gate

import (
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
