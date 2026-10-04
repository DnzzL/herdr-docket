package changelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var day = time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)

// repo writes a changelog.d/ and a CHANGELOG.md and returns both paths.
func repo(t *testing.T, existing string, frags map[string]string) (dir, path string) {
	t.Helper()
	root := t.TempDir()
	dir = filepath.Join(root, Dir)
	path = filepath.Join(root, "CHANGELOG.md")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range frags {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

const released = `# Changelog

What changed for someone using the plugin. Dates are release dates.

## v0.8.0 — 2026-10-02

- **Something shipped.**
`

// The whole point: two pull requests writing entries at the same time touch
// different files, so there is nothing for git to conflict on.
func TestTwoPullRequestsWriteDifferentFilesAndBothLand(t *testing.T) {
	dir, path := repo(t, released, map[string]string{
		"TASK-62.md": "- **A run that times out keeps its delivery.**",
		"TASK-63.md": "- **The merge a human owes announces itself.**",
	})
	got, err := Pending(dir, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"times out keeps its delivery", "merge a human owes"} {
		if !strings.Contains(got, want) {
			t.Errorf("pending lost %q:\n%s", want, got)
		}
	}
}

func TestFragmentsAreOrderedByFilename(t *testing.T) {
	dir, path := repo(t, released, map[string]string{
		"TASK-10.md": "- ten",
		"TASK-02.md": "- two",
		"TASK-31.md": "- thirtyone",
	})
	got, err := Pending(dir, path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "- two\n\n- ten\n\n- thirtyone"; got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Migration: the Unreleased section an older CHANGELOG still carries is not
// stranded — the next release folds it in under the fragments.
func TestAnOlderUnreleasedSectionIsFoldedInAfterTheFragments(t *testing.T) {
	existing := `# Changelog

## Unreleased

- **An entry written the old way.**

## v0.8.0 — 2026-10-02

- **Something shipped.**
`
	dir, path := repo(t, existing, map[string]string{"TASK-1.md": "- **A new entry.**"})
	section, err := Release(dir, path, "v0.9.0", day)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(section, "A new entry") || !strings.Contains(section, "written the old way") {
		t.Fatalf("the release dropped an entry:\n%s", section)
	}
	if strings.Index(section, "A new entry") > strings.Index(section, "written the old way") {
		t.Error("fragments should come first, newest at the top")
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "## Unreleased") {
		t.Error("the Unreleased section should be gone once its entries are released")
	}
	if !strings.Contains(string(raw), "## v0.8.0 — 2026-10-02") {
		t.Error("the older release sections must survive")
	}
}

// The heading has to be the one .github/workflows/release.yml greps for:
// it matches "## <tag> " at the start of a line, tag being the pushed vX.Y.Z.
func TestTheHeadingIsTheOneTheReleaseWorkflowGrepsFor(t *testing.T) {
	dir, path := repo(t, released, map[string]string{"TASK-1.md": "- **An entry.**"})
	if _, err := Release(dir, path, "v0.9.0", day); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "\n## v0.9.0 — 2026-10-04\n") {
		t.Fatalf("heading not in the shape release.yml finds:\n%s", raw)
	}
	if i, j := strings.Index(string(raw), "## v0.9.0"), strings.Index(string(raw), "## v0.8.0"); i > j {
		t.Error("the new release belongs above the previous one")
	}
	if !strings.HasPrefix(string(raw), "# Changelog\n\nWhat changed") {
		t.Errorf("the preamble must stay put:\n%s", raw)
	}
	// Markdown needs the break, and so does anyone reading the file.
	if !strings.Contains(string(raw), "dates.\n\n## v0.9.0") {
		t.Errorf("want a blank line between the preamble and the new heading:\n%s", raw)
	}
	if !strings.Contains(string(raw), "\n\n## v0.8.0") {
		t.Errorf("want a blank line before the previous release:\n%s", raw)
	}
}

func TestReleaseRemovesTheFragmentsItConsumed(t *testing.T) {
	dir, path := repo(t, released, map[string]string{
		"TASK-1.md": "- **One.**",
		"TASK-2.md": "- **Two.**",
		"README":    "not a fragment",
	})
	if _, err := Release(dir, path, "v0.9.0", day); err != nil {
		t.Fatal(err)
	}
	left, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0].Name() != "README" {
		t.Errorf("release should consume the .md fragments and nothing else, left: %v", left)
	}
}

func TestAVersionTheReleaseWorkflowCannotFindIsRefused(t *testing.T) {
	for _, v := range []string{"0.9.0", "v0.9", "release-9", "", "v0.9.0 "} {
		dir, path := repo(t, released, map[string]string{"TASK-1.md": "- **An entry.**"})
		if _, err := Release(dir, path, v, day); err == nil {
			t.Errorf("%q was accepted as a version", v)
		}
	}
}

func TestReleasingAVersionAlreadyInTheFileIsRefused(t *testing.T) {
	dir, path := repo(t, released, map[string]string{"TASK-1.md": "- **An entry.**"})
	if _, err := Release(dir, path, "v0.8.0", day); err == nil {
		t.Fatal("v0.8.0 is already a section; releasing it again should refuse")
	}
	raw, _ := os.ReadFile(path)
	if strings.Count(string(raw), "## v0.8.0") != 1 {
		t.Error("the refused release must not have touched the file")
	}
	if _, err := os.Stat(filepath.Join(dir, "TASK-1.md")); err != nil {
		t.Error("a refused release must leave the fragments alone")
	}
}

func TestReleasingNothingIsRefused(t *testing.T) {
	dir, path := repo(t, released, nil)
	if _, err := Release(dir, path, "v0.9.0", day); err == nil {
		t.Fatal("there is nothing to release; it should say so")
	}
}

// Before the first fragment is written there is no changelog.d/ at all.
// That is a real state, not a failure.
func TestAMissingFragmentDirHoldsNothing(t *testing.T) {
	_, path := repo(t, released, nil)
	got, err := Pending(filepath.Join(t.TempDir(), "absent"), path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("want nothing pending, got %q", got)
	}
}

// changelog.d/README.md documents the directory on the forge. It is the one
// .md in there that is not an entry, and a release must not announce it.
func TestTheDirectorysOwnReadmeIsNotAnEntry(t *testing.T) {
	dir, path := repo(t, released, map[string]string{
		"README.md": "# changelog.d\n\nOne file per pull request.",
		"TASK-1.md": "- **A real entry.**",
	})
	got, err := Pending(dir, path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "One file per pull request") {
		t.Errorf("the README was released as an entry:\n%s", got)
	}
	if !strings.Contains(got, "A real entry") {
		t.Errorf("the real entry went missing:\n%s", got)
	}
	if _, err := Release(dir, path, "v0.9.0", day); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
		t.Error("the release deleted the directory's README")
	}
}
