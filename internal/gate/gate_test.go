package gate

import (
	"strings"
	"testing"
)

func green() Input {
	return Input{
		PR:      PR{HeadSHA: "abc", PatchID: "p1", Checks: ChecksPass, Files: []string{"internal/x.go"}, State: "OPEN"},
		Verdict: Pass, VerdictPatch: "p1", Auto: true,
	}
}

func TestAPassOnTheCurrentPatchWithGreenChecksMerges(t *testing.T) {
	if merge, reason := Decide(green()); !merge {
		t.Fatalf("want merge, held: %s", reason)
	}
}

// Every guard holds the merge on its own, and says which one did.
func TestEachGuardHoldsTheMergeAndSaysWhy(t *testing.T) {
	cases := map[string]func(*Input){
		"verdict":    func(in *Input) { in.Verdict = Fail },
		"no verdict": func(in *Input) { in.Verdict = "" },
		"patch":      func(in *Input) { in.VerdictPatch = "p0" },
		"checks":     func(in *Input) { in.PR.Checks = ChecksFail },
		"no checks":  func(in *Input) { in.PR.Checks = ChecksNone },
		"critical":   func(in *Input) { in.Labels = []string{"bug", "Critical"} },
		"owned":      func(in *Input) { in.Owners = []string{"/internal/"} },
		"merge":      func(in *Input) { in.Auto = false },
		"state":      func(in *Input) { in.PR.State = "CLOSED" },
	}
	for name, mutate := range cases {
		in := green()
		mutate(&in)
		merge, reason := Decide(in)
		if merge || reason == "" {
			t.Errorf("%s: want a held merge with a reason, got merge=%v reason=%q", name, merge, reason)
		}
	}
}

func TestAnOwnedFileIsNamedInTheReason(t *testing.T) {
	in := green()
	in.PR.Files = []string{"README.md", "db/migrations/001.sql"}
	in.Owners = []string{"db/migrations/"}
	_, reason := Decide(in)
	if !strings.Contains(reason, "db/migrations/001.sql") {
		t.Fatalf("reason must name the file: %q", reason)
	}
}

func TestCodeownersPatterns(t *testing.T) {
	cases := []struct {
		pattern, file string
		want          bool
	}{
		{"*", "anything/at/all.go", true},
		{"*.sql", "db/x.sql", true},
		{"*.sql", "db/x.go", false},
		{"/docs/", "docs/a.md", true},
		{"/docs/", "src/docs/a.md", false},
		{"docs/", "src/docs/a.md", true},
		{"/auth", "auth/login.go", true},
		{"/auth", "authz/x.go", false},
		{"billing", "src/billing/x.go", true},
		{"/src/**/secret.go", "src/a/b/secret.go", true},
		{"/src/**/secret.go", "src/secret.go", true},
		{"/build/logs/*", "build/logs/x.log", true},
		{"/build/logs/*", "build/logs/deep/x.log", false},
		{"apps/*.go", "apps/x.go", true},
	}
	for _, c := range cases {
		if got := matches(c.pattern, c.file); got != c.want {
			t.Errorf("matches(%q, %q) = %v, want %v", c.pattern, c.file, got, c.want)
		}
	}
}

func TestParseCodeownersKeepsPatternsAndDropsComments(t *testing.T) {
	got := ParseCodeowners("# owners\n\n/db/ @me\n*.tf @ops @me\n  # indented\n")
	if strings.Join(got, ",") != "/db/,*.tf" {
		t.Fatalf("got %q", got)
	}
}
