package github

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// errNoGH is what ghToken says on a machine that has no gh logged in.
var errNoGH = errors.New("gh is not on PATH")

// ghOnPath puts a stand-in gh on PATH, so that asking the CLI is tested
// without a GitHub login on the machine running the tests.
func ghOnPath(t *testing.T, output string, ok bool) {
	t.Helper()
	dir := t.TempDir()
	if ok {
		script := filepath.Join(dir, "gh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' "+strconv.Quote(output)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
}

// gh is where a token with the right scopes already is, on a machine somebody
// has used GitHub from.
func TestGHTokenAsksTheCLI(t *testing.T) {
	ghOnPath(t, "gho_from-the-cli", true)
	got, err := ghToken()
	if err != nil {
		t.Fatalf("ghToken: %v", err)
	}
	if strings.TrimSpace(got) != "gho_from-the-cli" {
		t.Errorf("ghToken = %q, want what gh printed", got)
	}
}

func TestGHTokenSaysWhenThereIsNoGH(t *testing.T) {
	ghOnPath(t, "", false)
	if _, err := ghToken(); err == nil {
		t.Error("ghToken with no gh on PATH must fail")
	}
}

// inTempConfigDir puts the credentials file somewhere disposable, the way
// Basecamp's own tests do.
func inTempConfigDir(t *testing.T) {
	t.Helper()
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", t.TempDir())
}

// tokenSourceFor is a token source with the environment, gh and the file all
// saying something different, so the order between them is what a test reads.
func tokenSourceFor(t *testing.T, env, gh string) *tokenSource {
	t.Helper()
	return &tokenSource{
		store: defaultStore(),
		env:   func(string) string { return env },
		gh:    func() (string, error) { return gh, nil },
	}
}

// The environment is the deployment's word: it beats a developer's gh login,
// and it is the only one of the three that a CI runner has.
func TestTheEnvironmentWins(t *testing.T) {
	inTempConfigDir(t)
	if err := defaultStore().save("from-the-file"); err != nil {
		t.Fatal(err)
	}
	asked := false
	tokens := tokenSourceFor(t, "from-the-env", "from-gh")
	tokens.gh = func() (string, error) { asked = true; return "from-gh", nil }

	got, err := tokens.access()
	if err != nil {
		t.Fatalf("access: %v", err)
	}
	if got != "from-the-env" {
		t.Errorf("token = %q, want the environment's", got)
	}
	if asked {
		t.Error("gh was asked even though the environment answered")
	}
}

// gh is the thing that rotates a token, so when it is logged in it beats the
// copy a sign-in left behind.
func TestGHBeatsAStoredToken(t *testing.T) {
	inTempConfigDir(t)
	if err := defaultStore().save("from-the-file"); err != nil {
		t.Fatal(err)
	}
	tokens := tokenSourceFor(t, "", "from-gh")

	got, err := tokens.access()
	if err != nil {
		t.Fatalf("access: %v", err)
	}
	if got != "from-gh" {
		t.Errorf("token = %q, want gh's", got)
	}
}

func TestAStoredTokenIsTheLastResort(t *testing.T) {
	inTempConfigDir(t)
	if err := defaultStore().save("from-the-file"); err != nil {
		t.Fatal(err)
	}
	tokens := tokenSourceFor(t, "", "")

	got, err := tokens.access()
	if err != nil {
		t.Fatalf("access: %v", err)
	}
	if got != "from-the-file" {
		t.Errorf("token = %q, want the stored one", got)
	}
}

// A user with no token anywhere needs to be told all three ways to get one,
// and which of them the fleet tried.
func TestWithNoTokenTheFleetSaysHowToGetOne(t *testing.T) {
	inTempConfigDir(t)
	tokens := tokenSourceFor(t, "", "")
	tokens.gh = func() (string, error) { return "", errNoGH }

	_, err := tokens.access()
	if err == nil {
		t.Fatal("access with no token must fail")
	}
	for _, want := range []string{tokenEnv, "gh auth login", "herdr-docket auth github"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the message does not mention %q: %v", want, err)
		}
	}
}

// A failure to resolve is a fact about this moment, not about the process. The
// next tick must ask again.
func TestAFailureToResolveIsNotRemembered(t *testing.T) {
	inTempConfigDir(t)
	tokens := tokenSourceFor(t, "", "")
	tokens.gh = func() (string, error) { return "", errNoGH }
	if _, err := tokens.access(); err == nil {
		t.Fatal("access with no token must fail")
	}

	tokens.env = func(string) string { return "appeared" }
	got, err := tokens.access()
	if err != nil {
		t.Fatalf("access after the token appeared: %v", err)
	}
	if got != "appeared" {
		t.Errorf("token = %q, want the one that appeared", got)
	}
}

// The daemon builds a Source on every tick, so resolving is memoised. A token
// that was revoked is what invalidate is for.
func TestTheTokenIsResolvedOnceUntilItIsInvalidated(t *testing.T) {
	inTempConfigDir(t)
	resolved := 0
	tokens := &tokenSource{
		store: defaultStore(),
		gh:    func() (string, error) { return "", nil },
		env: func(string) string {
			resolved++
			if resolved == 1 {
				return "first"
			}
			return "second"
		},
	}

	for i := 0; i < 3; i++ {
		if _, err := tokens.access(); err != nil {
			t.Fatalf("access: %v", err)
		}
	}
	if resolved != 1 {
		t.Errorf("resolved %d times, want once", resolved)
	}
	tokens.invalidate()
	got, err := tokens.access()
	if err != nil {
		t.Fatalf("access after invalidate: %v", err)
	}
	if got != "second" {
		t.Errorf("token after invalidate = %q, want a fresh resolution", got)
	}
}

// Login is the sign-in the command line calls. A configuration it cannot use
// is refused before anything reaches GitHub.
func TestLoginRefusesAConfigurationItCannotUse(t *testing.T) {
	inTempConfigDir(t)
	if err := Login(Config{}, nil, "", io.Discard); err == nil {
		t.Error("Login with no board configured must fail")
	}
}

func TestLoginStoresTheTokenItWasHanded(t *testing.T) {
	inTempConfigDir(t)
	f := newFakeBoard()
	f.provisionAgent("dev")

	out := &bytes.Buffer{}
	if err := f.source(t).login([]string{"dev"}, "pat-123", out); err != nil {
		t.Fatalf("login: %v", err)
	}
	stored, err := defaultStore().load()
	if err != nil {
		t.Fatal(err)
	}
	if stored != "pat-123" {
		t.Errorf("the stored token is %q, want the one handed over", stored)
	}
	if !strings.Contains(out.String(), "Signed in to GitHub as "+testViewer) {
		t.Errorf("login did not say who it signed in as:\n%s", out)
	}
	info, err := os.Stat(defaultStore().file.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("credentials.yaml is %v, want 0600", info.Mode().Perm())
	}
}

// A token that GitHub refuses is not written down. A typo left in
// credentials.yaml would be found by every run after it, and every one of them
// would report GitHub's complaint instead of saying to sign in.
func TestLoginDoesNotStoreATokenGitHubRefused(t *testing.T) {
	inTempConfigDir(t)
	f := newFakeBoard()
	f.unauthorized = 10 // every request, so the viewer check cannot pass

	err := f.source(t).login([]string{"dev"}, "pat-typo", io.Discard)
	if err == nil {
		t.Fatal("login with a token GitHub refuses must fail")
	}
	stored, loadErr := defaultStore().load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if stored != "" {
		t.Errorf("the refused token was stored as %q, want nothing written", stored)
	}
}

// When gh is there and says no, its own sentence is the useful part of the
// failure: it names the command that fixes it.
func TestGHTokenSurfacesWhatGHSaid(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	body := "#!/bin/sh\necho 'To get started with GitHub CLI, please run: gh auth login' >&2\nexit 1\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	_, err := ghToken()
	if err == nil || !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("ghToken = %v, want gh's own complaint", err)
	}
}

// Signing in is where the routing field is made, so that no run ever has to
// change the board's schema underneath a person who is editing it.
func TestLoginCreatesTheRoutingField(t *testing.T) {
	inTempConfigDir(t)
	f := newFakeBoard() // a board with Status and no way to route
	out := &bytes.Buffer{}

	if err := f.source(t).login([]string{"reviewer", "dev"}, "", out); err != nil {
		t.Fatalf("login: %v", err)
	}
	if got := strings.Join(f.operations(), " "); got != "Viewer Project CreateField" {
		t.Fatalf("login made the requests %v, want it to look the board over and make the field", got)
	}
	field := f.field("Agent")
	if field == nil {
		t.Fatal("no Agent field was made")
	}
	var names []string
	for _, o := range field.Options {
		names = append(names, o.Name)
		if o.Color == "" || o.Color == "GRAY" {
			t.Errorf("option %q has colour %q, want one GitHub would accept", o.Name, o.Color)
		}
	}
	if strings.Join(names, ",") != "dev,reviewer" {
		t.Errorf("the field carries %v, want one option per agent", names)
	}
	if !strings.Contains(out.String(), "Created the Agent field") {
		t.Errorf("login did not say what it made:\n%s", out)
	}
}

// An option that is already there keeps its identity. Rewriting it without its
// id would make a new option and clear every item that was routed with the old
// one — silently, and for work already in flight.
func TestLoginAddsOnlyTheMissingOptionsAndKeepsTheRest(t *testing.T) {
	inTempConfigDir(t)
	f := newFakeBoard()
	f.provisionAgent("pm")
	pmID := f.field("Agent").Options[0].ID
	n := f.add("already routed", "")
	f.itemOf(n).agent = "pm"

	out := &bytes.Buffer{}
	if err := f.source(t).login([]string{"dev", "pm"}, "", out); err != nil {
		t.Fatalf("login: %v", err)
	}
	if got := strings.Join(f.operations(), " "); got != "Viewer Project UpdateField" {
		t.Fatalf("login made the requests %v, want it to update the field", got)
	}
	var carried []string
	for _, o := range f.field("Agent").Options {
		if o.Name == "pm" && o.ID != pmID {
			t.Errorf("the existing option pm was rewritten as %q, want its id %q kept", o.ID, pmID)
		}
		carried = append(carried, o.Name)
	}
	if strings.Join(carried, ",") != "pm,dev" {
		t.Errorf("the field carries %v, want the existing option and the new one", carried)
	}
	if got := f.itemOf(n).agent; got != "pm" {
		t.Errorf("the routed item reads %q after sign-in, want its routing key untouched", got)
	}
	if !strings.Contains(out.String(), "added dev") {
		t.Errorf("login did not say what it added:\n%s", out)
	}
}

func TestLoginWritesNothingWhenEveryOptionIsThere(t *testing.T) {
	inTempConfigDir(t)
	f := newFakeBoard()
	f.provisionAgent("dev")

	if err := f.source(t).login([]string{"dev"}, "", io.Discard); err != nil {
		t.Fatalf("login: %v", err)
	}
	if got := strings.Join(f.operations(), " "); got != "Viewer Project" {
		t.Errorf("login made the requests %v, want nothing written", got)
	}
}

// Routing needs a single select. A text field with the right name is a
// configuration mistake, and it is said so rather than written over.
func TestLoginRefusesAFieldThatCannotCarryTheRoutingKey(t *testing.T) {
	inTempConfigDir(t)
	f := newFakeBoard()
	f.fields = append(f.fields, &fakeField{typename: "ProjectV2Field", id: f.id("PVTF"), name: "Agent"})

	err := f.source(t).login([]string{"dev"}, "", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "not a single select") {
		t.Fatalf("login said %v, want it to refuse the field it cannot route with", err)
	}
	if got := strings.Join(f.operations(), " "); got != "Viewer Project" {
		t.Errorf("login made the requests %v, want nothing written", got)
	}
}

// With no agents described there is nothing to route to, and an empty Agent
// field would be a board that silently swallows every task.
func TestLoginRefusesWhenNoAgentsAreDescribed(t *testing.T) {
	inTempConfigDir(t)
	f := newFakeBoard()

	err := f.source(t).login(nil, "", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "no agents to route to") {
		t.Fatalf("login said %v, want it to ask for the agents first", err)
	}
	if got := strings.Join(f.operations(), " "); got != "Viewer Project" {
		t.Errorf("login made the requests %v, want nothing written", got)
	}
}

func TestLoginRefusesATokenThatAnswersNothing(t *testing.T) {
	inTempConfigDir(t)
	f := newFakeBoard()
	f.viewer = ""

	err := f.source(t).login([]string{"dev"}, "", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "no login") {
		t.Fatalf("login said %v, want it to question the token", err)
	}
}

// The Status column is for the person watching the board. Nothing the fleet
// decides reads it, so a board that cannot show work in hand is a note rather
// than a failure — but it is said out loud, because a run that shows as
// nothing looks like a run that did nothing.
func TestLoginSaysWhatTheBoardWillNotShow(t *testing.T) {
	t.Run("no column at all", func(t *testing.T) {
		inTempConfigDir(t)
		f := newFakeBoard()
		f.fields = nil
		f.provisionAgent("dev")

		out := &bytes.Buffer{}
		if err := f.source(t).login([]string{"dev"}, "", out); err != nil {
			t.Fatalf("login: %v", err)
		}
		if !strings.Contains(out.String(), "in hand") || !strings.Contains(out.String(), "status_field") {
			t.Errorf("login did not say the board cannot show work in hand:\n%s", out)
		}
	})

	t.Run("no In Progress option", func(t *testing.T) {
		inTempConfigDir(t)
		f := newFakeBoard()
		f.fields = []*fakeField{f.newField("Status", "Todo", "Done")}
		f.provisionAgent("dev")

		out := &bytes.Buffer{}
		if err := f.source(t).login([]string{"dev"}, "", out); err != nil {
			t.Fatalf("login: %v", err)
		}
		if !strings.Contains(out.String(), `no "In Progress" option`) {
			t.Errorf("login did not say which column is missing:\n%s", out)
		}
	})

	t.Run("a column that is not a single select", func(t *testing.T) {
		inTempConfigDir(t)
		f := newFakeBoard()
		f.fields = []*fakeField{{typename: "ProjectV2Field", id: f.id("PVTF"), name: "Status"}}
		f.provisionAgent("dev")

		out := &bytes.Buffer{}
		if err := f.source(t).login([]string{"dev"}, "", out); err != nil {
			t.Fatalf("login: %v", err)
		}
		if !strings.Contains(out.String(), "not a single select") {
			t.Errorf("login did not say the column is the wrong shape:\n%s", out)
		}
	})

	t.Run("a board that shows everything", func(t *testing.T) {
		inTempConfigDir(t)
		f := newFakeBoard()
		f.provisionAgent("dev")

		out := &bytes.Buffer{}
		if err := f.source(t).login([]string{"dev"}, "", out); err != nil {
			t.Fatalf("login: %v", err)
		}
		if strings.Contains(out.String(), "Note:") {
			t.Errorf("a board that is ready for a run should carry no note:\n%s", out)
		}
	})
}
