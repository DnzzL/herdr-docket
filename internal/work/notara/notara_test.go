package notara

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	credstore "github.com/DnzzL/herdr-docket/internal/credentials"
	"github.com/DnzzL/herdr-docket/internal/work"
	"github.com/DnzzL/herdr-docket/internal/work/worktest"
)

// testVocabulary is Notara's own backlog in the fleet's terms, as fleet.yaml
// will say it: the triage words stay the humans'.
var testVocabulary = work.Vocabulary{
	Todo:       "ready-for-agent",
	InProgress: "in-progress",
	Done:       "done",
	Failed:     "failed",
	Blocked:    "blocked",
}

var (
	_ work.Source          = (*Source)(nil)
	_ work.Phaser          = (*Source)(nil)
	_ work.Assigner        = (*Source)(nil)
	_ work.TodoStarter     = (*Source)(nil)
	_ work.CriterionWriter = (*Source)(nil)
	_ work.BaseBrancher    = (*Source)(nil)
)

func TestTheContract(t *testing.T) {
	worktest.Run(t, func(t *testing.T) work.Source { return newFakeNotara(t).source(t) })
}

// The status word is the whole lifecycle: a whitelist of what is open, the
// blocked word as parked, an ending read back as its verdict. A triage
// column is none of the fleet's business, and a trashed card is not a card.
func TestACardsStatusIsReadThroughTheVocabulary(t *testing.T) {
	f := newFakeNotara(t)
	for _, r := range []fakeRecord{
		{Title: "ready", Fields: map[string]any{"Status": "ready-for-agent", "Agent": "dev", "Priority": "high", "Labels": []any{"critical"}}},
		{Title: "triage", Fields: map[string]any{"Status": "needs-triage"}},
		{Title: "held", Fields: map[string]any{"Status": "blocked"}},
		{Title: "broke", Fields: map[string]any{"Status": "failed"}},
		{Title: "nothing"},
		{Title: "trashed", Deleted: true, Fields: map[string]any{"Status": "ready-for-agent"}},
	} {
		f.seed(r)
	}
	items, err := f.source(t).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 {
		t.Fatalf("listed %d cards, want 5 (a trashed card is not work): %+v", len(items), items)
	}
	want := []struct {
		id, phase string
		open      bool
		blocked   bool
		verdict   work.Verdict
	}{
		{"NOT-1", "To Do", true, false, ""},
		{"NOT-2", "needs-triage", false, false, ""},
		{"NOT-3", "Blocked", false, true, work.Blocked},
		{"NOT-4", "Failed", false, false, work.Failed},
		{"NOT-5", "", false, false, ""},
	}
	for i, w := range want {
		it := items[i]
		if it.ID != w.id || it.Phase != w.phase || it.Open != w.open || it.Blocked != w.blocked || it.Verdict != w.verdict {
			t.Errorf("item %d = {%s %q open=%v blocked=%v %q}, want %+v", i, it.ID, it.Phase, it.Open, it.Blocked, it.Verdict, w)
		}
	}
	first := items[0]
	if first.Assignee != "dev" || first.Priority != 3 || !slices.Equal(first.Labels, []string{"critical"}) || first.CreatedAt == "" {
		t.Errorf("routing, rank and labels lost: %+v", first)
	}
}

func TestGetReadsTheBodyTheCriteriaAndTheThread(t *testing.T) {
	f := newFakeNotara(t)
	f.seed(fakeRecord{
		Title:  "Wire it",
		Body:   "Why it matters.",
		Fields: map[string]any{"Status": "done"},
		Criteria: []fakeCriterion{
			{ID: "b1:0", Text: "compiles", Checked: true},
			{ID: "b2:0", Text: "tested"},
		},
		Comments: []map[string]any{
			{"id": "c1", "authorId": "u1", "body": "@thomas which locale?", "keyName": "fleet"},
			{"id": "c2", "authorId": "u1", "body": "fr-FR", "keyName": nil},
		},
	})
	it, err := f.source(t).Get("NOT-1")
	if err != nil {
		t.Fatal(err)
	}
	if it.Body != "Why it matters." || it.Verdict != work.Done || it.Open {
		t.Errorf("task = %+v", it)
	}
	if len(it.Criteria) != 2 || it.Criteria[0] != (work.Criterion{Index: 1, Text: "compiles", Checked: true}) || it.Criteria[1].Index != 2 {
		t.Errorf("criteria = %+v", it.Criteria)
	}
	if it.Notes != "fleet: @thomas which locale?\n\nfr-FR" {
		t.Errorf("notes = %q", it.Notes)
	}
}

// Notara's select has no default: a card created with no status is invisible
// to the fleet forever. Plain Create therefore files into the pickup word too.
func TestNewWorkLandsInThePickupStatus(t *testing.T) {
	f := newFakeNotara(t)
	s := f.source(t)
	for name, create := range map[string]func(string, string, string) (string, error){
		"Create": s.Create, "CreateTodo": s.CreateTodo,
	} {
		id, err := create("Follow up "+name, "", "")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		rec := f.get(id)
		if rec.Fields["Status"] != "ready-for-agent" {
			t.Errorf("%s filed %s under %v", name, id, rec.Fields["Status"])
		}
		if _, ok := rec.Fields["Agent"]; ok {
			t.Errorf("%s with no assignee wrote an Agent: %v", name, rec.Fields["Agent"])
		}
	}
}

// An agent the Agent select does not offer is Notara's 422. The fleet must
// hear Notara's reason and what fixes it, not a bare status.
func TestAnAgentTheDatabaseDoesNotOfferIsRefusedWithTheFix(t *testing.T) {
	f := newFakeNotara(t)
	_, err := f.source(t).Create("t", "", "ghost")
	if err == nil {
		t.Fatal("a card for an agent Notara does not offer was created")
	}
	for _, want := range []string{"422", `"ghost"`, "auth notara"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %s", err, want)
		}
	}
}

func TestEveryVerdictWritesItsOwnWord(t *testing.T) {
	for v, word := range map[work.Verdict]string{work.Done: "done", work.Failed: "failed", work.Blocked: "blocked"} {
		f := newFakeNotara(t)
		f.seed(fakeRecord{Title: "t", Fields: map[string]any{"Status": "in-progress"}})
		if err := f.source(t).Close("NOT-1", v); err != nil {
			t.Fatalf("Close(%s): %v", v, err)
		}
		if got := f.get("NOT-1").Fields["Status"]; got != word {
			t.Errorf("Close(%s) wrote %v, want %s", v, got, word)
		}
	}
}

func TestAnUnknownVerdictWritesNothing(t *testing.T) {
	f := newFakeNotara(t)
	f.seed(fakeRecord{Title: "t", Fields: map[string]any{"Status": "ready-for-agent"}})
	if err := f.source(t).Close("NOT-1", "probably"); err == nil {
		t.Fatal("an unknown verdict was accepted")
	}
	if f.writeCount() != 0 {
		t.Errorf("a refused verdict made %d writes", f.writeCount())
	}
}

func TestPhaseAndAssignWriteTheirFields(t *testing.T) {
	f := newFakeNotara(t)
	f.seed(fakeRecord{Title: "t", Fields: map[string]any{"Status": "ready-for-agent", "Agent": "dev"}})
	s := f.source(t)
	if err := s.SetPhase("NOT-1", work.PhaseInProgress); err != nil {
		t.Fatal(err)
	}
	if err := s.Assign("NOT-1", "reviewer"); err != nil {
		t.Fatal(err)
	}
	if got := f.get("NOT-1").Fields; got["Status"] != "in-progress" || got["Agent"] != "reviewer" {
		t.Errorf("fields = %v", got)
	}
	if err := s.Assign("NOT-1", ""); err != nil {
		t.Fatalf("clearing the agent: %v", err)
	}
	if got, ok := f.get("NOT-1").Fields["Agent"]; ok {
		t.Errorf("Assign(\"\") left the agent %v", got)
	}
	if err := s.SetPhase("NOT-1", work.PhaseTodo); err == nil {
		t.Error("the fleet writes one phase; another must be refused, not guessed")
	}
}

// A queue that has no word for work in hand is quieter, not broken.
func TestNoInProgressWordMeansNoPhaseWrite(t *testing.T) {
	f := newFakeNotara(t)
	f.seed(fakeRecord{Title: "t", Fields: map[string]any{"Status": "ready-for-agent"}})
	v := testVocabulary
	v.InProgress = ""
	s, err := New(Config{URL: f.srv.URL, Workspace: fakeWorkspace, Database: fakeDatabase}, v, "")
	if err != nil {
		t.Fatal(err)
	}
	s.api.token = func() (string, error) { return fakeKey, nil }
	if err := s.SetPhase("NOT-1", work.PhaseInProgress); err != nil {
		t.Fatal(err)
	}
	if f.writeCount() != 0 {
		t.Errorf("SetPhase with no in_progress word wrote %d times", f.writeCount())
	}
}

func TestCriteriaAreAppended(t *testing.T) {
	f := newFakeNotara(t)
	f.seed(fakeRecord{Title: "t", Criteria: []fakeCriterion{{ID: "b:0", Text: "first"}}})
	if err := f.source(t).WriteCriteria("NOT-1", []string{"second"}); err != nil {
		t.Fatal(err)
	}
	if got := f.get("NOT-1").Criteria; len(got) != 2 || got[1].Text != "second" {
		t.Errorf("criteria = %+v", got)
	}
}

// A hosted queue has no repo to derive a base from: only the source's own
// worktree_base: answers, and without one the runner inherits, as before.
func TestTheBaseBranchIsTheConfiguredOneOrNone(t *testing.T) {
	f := newFakeNotara(t)
	if _, err := f.source(t).BaseBranch("NOT-1"); err == nil {
		t.Error("a Notara queue with no worktree_base named a branch")
	}
	s, err := New(Config{URL: f.srv.URL, Workspace: fakeWorkspace, Database: fakeDatabase}, testVocabulary, "origin/main")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.BaseBranch("NOT-1"); err != nil || got != "origin/main" {
		t.Errorf("BaseBranch = %q, %v", got, err)
	}
}

func TestConfigurationIsCheckedAtStartup(t *testing.T) {
	ok := Config{URL: "http://localhost:3100", Workspace: "w", Database: "d"}
	for name, tc := range map[string]struct {
		c     Config
		v     work.Vocabulary
		names string
	}{
		"no url":        {Config{Workspace: "w", Database: "d"}, testVocabulary, "url"},
		"no workspace":  {Config{URL: ok.URL, Database: "d"}, testVocabulary, "workspace"},
		"no database":   {Config{URL: ok.URL, Workspace: "w"}, testVocabulary, "database"},
		"no statuses":   {ok, work.Vocabulary{}, "statuses"},
		"half statuses": {ok, work.Vocabulary{Todo: "ready-for-agent", Done: "done"}, "failed"},
	} {
		_, err := New(tc.c, tc.v, "")
		if err == nil || !strings.Contains(err.Error(), tc.names) {
			t.Errorf("%s: err = %v, want it to name %s", name, err, tc.names)
		}
	}
}

func TestAWrongKeySaysHowToSignIn(t *testing.T) {
	f := newFakeNotara(t)
	s := f.source(t)
	s.api.token = func() (string, error) { return "ntr_wrong", nil }
	_, err := s.List()
	if err == nil || !strings.Contains(err.Error(), "auth notara") {
		t.Fatalf("err = %v, want it to point at `auth notara`", err)
	}
}

func TestNoKeyAtAllSaysHowToSignIn(t *testing.T) {
	t.Setenv(tokenEnv, "")
	tokens := &tokenSource{store: credstore.File{Path: filepath.Join(t.TempDir(), "credentials.yaml")}}
	if _, err := tokens.access(); err == nil || !strings.Contains(err.Error(), "auth notara") {
		t.Fatalf("err = %v", err)
	}
}

// Signing in is where the database learns the fleet's words: every agent
// and every status the fleet writes becomes an option, and the options a
// human already had stay. A key Notara refuses is never written down.
func TestSignInStoresTheKeyAndProvisionsTheOptions(t *testing.T) {
	t.Setenv(tokenEnv, "")
	f := newFakeNotara(t)
	f.fields[0].Options = []string{"needs-triage", "ready-for-agent", "done"}
	store := credstore.File{Path: filepath.Join(t.TempDir(), "credentials.yaml")}
	cfg := Config{URL: f.srv.URL, Workspace: fakeWorkspace, Database: fakeDatabase}

	var out bytes.Buffer
	if err := login(cfg, testVocabulary, []string{"notara-dev", "dev"}, "ntr_wrong", store, &out); err == nil {
		t.Fatal("a key Notara refused was accepted")
	}
	if tok, _ := (&tokenSource{store: store}).fromStore(); tok != "" {
		t.Fatal("a refused key was stored")
	}

	if err := login(cfg, testVocabulary, []string{"notara-dev", "dev"}, fakeKey, store, &out); err != nil {
		t.Fatalf("login: %v\n%s", err, out.String())
	}
	if tok, _ := (&tokenSource{store: store}).fromStore(); tok != fakeKey {
		t.Errorf("stored key = %q", tok)
	}
	if got := f.field("Status").Options; !slices.Equal(got, []string{"needs-triage", "ready-for-agent", "done", "in-progress", "blocked", "failed"}) {
		t.Errorf("status options = %v", got)
	}
	if got := f.field("Agent").Options; !slices.Equal(got, []string{"thomas", "dev", "reviewer", "notara-dev"}) {
		t.Errorf("agent options = %v", got)
	}

	// Twice is once: nothing left to add, nothing written.
	before := f.writeCount()
	if err := login(cfg, testVocabulary, []string{"notara-dev", "dev"}, "", store, &out); err != nil {
		t.Fatal(err)
	}
	if f.writeCount() != before {
		t.Errorf("a second sign-in wrote %d times", f.writeCount()-before)
	}
}
