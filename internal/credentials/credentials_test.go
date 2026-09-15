package credentials

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// section is a stand-in for a backend's block: the package only ever sees the
// name and the value, never the shape.
type section struct {
	Token string `yaml:"token"`
	Note  string `yaml:"note,omitempty"`
}

func inTempFile(t *testing.T) File {
	t.Helper()
	return File{Path: filepath.Join(t.TempDir(), "credentials.yaml")}
}

func TestSaveThenLoadRoundTripsASection(t *testing.T) {
	f := inTempFile(t)
	want := section{Token: "ghp_secret", Note: "two\nlines"}
	if err := f.Save("github", want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	var got section
	if err := f.Load("github", &got); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Fatalf("loaded %+v, want %+v", got, want)
	}
}

// The reason this package exists. A backend that saves its own section must
// not erase another's — one file, several writers, and each of them only
// knows its own shape.
func TestSavingOneSectionLeavesTheOthersAlone(t *testing.T) {
	f := inTempFile(t)
	if err := f.Save("basecamp", section{Token: "bc"}); err != nil {
		t.Fatalf("Save basecamp: %v", err)
	}
	if err := f.Save("github", section{Token: "gh"}); err != nil {
		t.Fatalf("Save github: %v", err)
	}
	if err := f.Save("basecamp", section{Token: "bc-refreshed"}); err != nil {
		t.Fatalf("Save basecamp again: %v", err)
	}

	var bc, gh section
	if err := f.Load("basecamp", &bc); err != nil {
		t.Fatalf("Load basecamp: %v", err)
	}
	if err := f.Load("github", &gh); err != nil {
		t.Fatalf("Load github: %v", err)
	}
	if bc.Token != "bc-refreshed" {
		t.Errorf("basecamp = %q, want the refreshed token", bc.Token)
	}
	if gh.Token != "gh" {
		t.Errorf("github = %q, want it untouched by basecamp's save", gh.Token)
	}
}

// A section written by a newer version of the fleet, or by hand, survives a
// save by a version that has never heard of it.
func TestSavingKeepsSectionsItCannotRead(t *testing.T) {
	f := inTempFile(t)
	if err := os.WriteFile(f.Path, []byte("future:\n  shape: [1, 2]\n  nested:\n    deep: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := f.Save("github", section{Token: "gh"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(f.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"future:", "shape:", "deep: true", "github:", "gh"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("saved file lost %q:\n%s", want, raw)
		}
	}
}

func TestAMissingFileAndAnAbsentSectionAreNotErrors(t *testing.T) {
	f := inTempFile(t)
	var got section
	if err := f.Load("github", &got); err != nil {
		t.Fatalf("a fleet that has never logged in should load a zero value, not fail: %v", err)
	}
	if got != (section{}) {
		t.Fatalf("loaded %+v from nowhere", got)
	}
	if err := f.Save("github", section{Token: "gh"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	var missing section
	if err := f.Load("basecamp", &missing); err != nil {
		t.Fatalf("Load of a section that is not there: %v", err)
	}
	if missing != (section{}) {
		t.Fatalf("loaded %+v from a section that does not exist", missing)
	}
}

// A secret file that was once world-readable must not stay that way. WriteFile
// only applies its mode when it creates the file.
func TestSavingTightensTheFileMode(t *testing.T) {
	f := inTempFile(t)
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.Path, []byte("github:\n  token: leaked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.Save("github", section{Token: "gh"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(f.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("credentials.yaml is %o, want 600", got)
	}
}
