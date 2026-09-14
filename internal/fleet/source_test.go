package fleet

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/DnzzL/herdr-docket/internal/work"
	"github.com/DnzzL/herdr-docket/internal/work/backlogmd"
	"github.com/DnzzL/herdr-docket/internal/work/basecamp"
)

// A fleet that names no source at all still works: Backlog.md in the fleet
// dir is what every fleet was before there was a choice, and the choice must
// not break it.
func TestASourceBlockIsOptional(t *testing.T) {
	writeFleetYAML(t, "dir: /tmp/somewhere\n")
	s, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	src, err := NewSource(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := src.(*backlogmd.Source); !ok {
		t.Fatalf("a fleet with no source block must mean Backlog.md, got %T", src)
	}
}

func TestASourceBlockNamesTheKind(t *testing.T) {
	writeFleetYAML(t, "dir: /tmp/somewhere\nsource:\n  kind: backlogmd\n")
	s, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.Source.Kind != kindBacklogmd {
		t.Fatalf("kind = %q, want %q", s.Source.Kind, kindBacklogmd)
	}
	if src, err := NewSource(s); err != nil {
		t.Fatal(err)
	} else if _, ok := src.(*backlogmd.Source); !ok {
		t.Fatalf("got %T", src)
	}
}

// A kind nobody implements is a fleet pointed at nothing. Falling back to
// Backlog.md would run every task in the wrong place and look healthy doing
// it, which is the failure mode this whole package exists to avoid.
func TestAnUnknownSourceKindIsRefused(t *testing.T) {
	writeFleetYAML(t, "source:\n  kind: trello\n")
	s, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewSource(s)
	if err == nil {
		t.Fatal("want an error for a source kind nothing implements")
	}
	if !strings.Contains(err.Error(), "trello") {
		t.Fatalf("the error must name the kind that is not understood, got %q", err)
	}
}

// Only a queue that lives in the fleet dir is the fleet's to lay down. A
// remote one is configured wherever it lives; creating a local project for it
// would be scaffolding a queue nothing reads.
func TestOnlyALocalQueueIsScaffolded(t *testing.T) {
	for _, tc := range []struct {
		kind string
		want bool
	}{
		{"", true}, // no block at all: the default, Backlog.md
		{"backlogmd", true},
		{"basecamp", false},
	} {
		if got := (SourceConfig{Kind: tc.kind}).local(); got != tc.want {
			t.Errorf("kind %q: local = %v, want %v", tc.kind, got, tc.want)
		}
	}
	// A Backlog.md project of its own is a repo that already exists. Laying
	// one down there would scaffold over somebody else's backlog.
	if (SourceConfig{Kind: "backlogmd", Dir: "/srv/notion-alt"}).local() {
		t.Error("a source naming its own project is not the fleet's to scaffold")
	}
	if q := (Settings{Dir: "/tmp/fleet", Sources: map[string]SourceConfig{
		"notara":  {Dir: "/srv/notion-alt"},
		"dishnow": {Dir: "/srv/dishnow-v2"},
	}}).ownQueues(); len(q) != 0 {
		t.Errorf("a fleet whose every queue is elsewhere creates none, got %d", len(q))
	}
	if q := (Settings{Dir: "/tmp/fleet"}).ownQueues(); len(q) != 1 {
		t.Errorf("a fleet with its own queue still creates it, got %d", len(q))
	}
}

// A source block that names Basecamp carries a block of its own, and the
// adapter owns that block's shape — this package only hands it over.
func TestASourceBlockCarriesTheBasecampConfig(t *testing.T) {
	writeFleetYAML(t, `source:
  kind: basecamp
  basecamp:
    account_id: "999"
    lists:
      dev: "111"
      pm: "222"
`)
	s, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Source.Basecamp.AccountID; got != "999" {
		t.Fatalf("account_id = %q, want 999", got)
	}
	if got := s.Source.Basecamp.Lists["pm"]; got != "222" {
		t.Fatalf("lists.pm = %q, want 222", got)
	}
	if src, err := NewSource(s); err != nil {
		t.Fatal(err)
	} else if _, ok := src.(*basecamp.Source); !ok {
		t.Fatalf("got %T", src)
	}
}

// A Basecamp fleet that names no lists has no routing table at all, so it can
// only be misrouted. Better to refuse it at startup than at the first poll.
func TestABasecampFleetWithNothingToRouteWithIsRefused(t *testing.T) {
	writeFleetYAML(t, "source:\n  kind: basecamp\n  basecamp:\n    account_id: \"999\"\n")
	s, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewSource(s); err == nil {
		t.Fatal("want an error for a Basecamp fleet with no lists")
	}
}

// `auth` only means something for a queue that lives somewhere else. A local
// one has no account behind it, and saying so beats a confusing no-op.
func TestOnlyAHostedQueueCanBeSignedInTo(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind string
		want string
	}{
		{"the default queue is local", "", "local queue"},
		{"backlogmd is local", "backlogmd", "local queue"},
		{"a queue nothing implements", "trello", "trello"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Auth(tc.kind, io.Discard)
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// basecamp is routed to the real flow, which refuses before it opens a
// listener when there is no application to authorize against.
func TestAuthingBasecampReachesTheBasecampFlow(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", t.TempDir())
	t.Setenv("HERDR_DOCKET_BASECAMP_CLIENT_ID", "")
	err := Auth("basecamp", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "Launchpad client id") {
		t.Fatalf("err = %v, want the missing-client-id message from the basecamp flow", err)
	}
}

// writeFleetYAML points the config dir at a temp dir holding one fleet.yaml.
func writeFleetYAML(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fleet.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", dir)
}

// Several sources: are a composite, one queue per name, and the names come
// back in a stable order so two projects' tasks do not reshuffle between
// polls.
func TestSeveralSourcesBecomeAComposite(t *testing.T) {
	writeFleetYAML(t, `sources:
  myapp:
    kind: backlogmd
  bc:
    kind: basecamp
    basecamp:
      account_id: "999"
      lists:
        dev: "111"
`)
	s, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	src, err := NewSource(s)
	if err != nil {
		t.Fatal(err)
	}
	ms, ok := src.(work.MultiSource)
	if !ok {
		t.Fatalf("several sources must be a composite, got %T", src)
	}
	if names := ms.Names(); len(names) != 2 || names[0] != "bc" || names[1] != "myapp" {
		t.Fatalf("Names = %v, want the sorted [bc myapp]", names)
	}
}

// A composite is a composite even with one name: the prefix appears the
// moment a queue has a name, and one name is still a name.
func TestOneNamedSourceIsAComposite(t *testing.T) {
	writeFleetYAML(t, "sources:\n  solo:\n    kind: backlogmd\n")
	s, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	src, err := NewSource(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := src.(work.MultiSource); !ok {
		t.Fatalf("sources: with one entry is still a composite, got %T", src)
	}
}

// source: and sources: together is two arrangements of queues in one file,
// which is a decision the fleet refuses to make for you.
func TestSourceAndSourcesTogetherAreRefused(t *testing.T) {
	writeFleetYAML(t, "source:\n  kind: backlogmd\nsources:\n  a:\n    kind: backlogmd\n")
	if _, err := LoadSettings(); err == nil {
		t.Fatal("source: and sources: together must be refused")
	}
}

// A bad kind inside a named source names the source that is broken, so one
// wrong queue in a composite is obvious rather than a mystery to bisect.
func TestABadKindInsideANamedSourceNamesTheSource(t *testing.T) {
	writeFleetYAML(t, "sources:\n  myapp:\n    kind: trello\n")
	s, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewSource(s)
	if err == nil || !strings.Contains(err.Error(), "myapp") || !strings.Contains(err.Error(), "trello") {
		t.Fatalf("the error must name the source and the kind, got %v", err)
	}
}

// The point of several sources: each one is a different project's own
// backlog. Without a dir of its own every local source resolves to the fleet
// dir, and a fleet "working two projects" quietly works one of them twice.
func TestALocalSourceCanNameItsOwnProject(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		dir  string
		want string
	}{
		{"no dir is the fleet's own queue", "", "/tmp/fleet"},
		{"absolute is taken as it stands", "/srv/notion-alt", "/srv/notion-alt"},
		{"~ expands", "~/Projects/notion-alt", filepath.Join(home, "Projects/notion-alt")},
		{"relative hangs off the fleet dir", "projects/dishnow", "/tmp/fleet/projects/dishnow"},
	} {
		got := SourceConfig{Dir: tc.dir}.dirFrom("/tmp/fleet")
		if got != tc.want {
			t.Errorf("%s: dirFrom = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A vocabulary is refused where it is written, not where it is used. Half of
// one is a fleet that picks work up and cannot put it down: the run ends, the
// status never moves, and the daemon picks the same task straight back up.
func TestAHalfWrittenVocabularyIsRefused(t *testing.T) {
	writeFleetYAML(t, `dir: /tmp/fleet
sources:
  notara:
    kind: backlogmd
    dir: /srv/notion-alt
    statuses:
      todo: ready-for-agent
      done: done
`)
	s, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewSource(s)
	if err == nil {
		t.Fatal("want an error for a vocabulary with no word for a failed run")
	}
	for _, want := range []string{"notara", "failed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error must name %q, got %q", want, err)
		}
	}
}

// The whole arrangement, read end to end: two projects, each with its own
// directory and its own words, behind one fleet.
func TestSeveralSourcesEachCarryTheirOwnProjectAndWords(t *testing.T) {
	writeFleetYAML(t, `dir: /tmp/fleet
sources:
  notara:
    kind: backlogmd
    dir: /srv/notion-alt
    statuses:
      todo: ready-for-agent
      done: done
      failed: ready-for-human
      blocked: needs-info
  dishnow:
    kind: backlogmd
    dir: /srv/dishnow-v2
    statuses:
      todo: ready for agent
      in_progress: In Progress
      done: Done
      failed: wontfix
      blocked: needs human validation
`)
	s, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Sources["notara"].Dir; got != "/srv/notion-alt" {
		t.Errorf("notara dir = %q", got)
	}
	if got := s.Sources["notara"].Statuses.Todo; got != "ready-for-agent" {
		t.Errorf("notara todo = %q", got)
	}
	// notara has no word for work in hand, and that is allowed: the phase is
	// display only.
	if got := s.Sources["notara"].Statuses.InProgress; got != "" {
		t.Errorf("notara in_progress = %q, want none", got)
	}
	if got := s.Sources["dishnow"].Statuses.Blocked; got != "needs human validation" {
		t.Errorf("dishnow blocked = %q", got)
	}
	src, err := NewSource(s)
	if err != nil {
		t.Fatal(err)
	}
	ms, ok := src.(work.MultiSource)
	if !ok {
		t.Fatalf("several sources must be one MultiSource, got %T", src)
	}
	if want := []string{"dishnow", "notara"}; !reflect.DeepEqual(ms.Names(), want) {
		t.Fatalf("names = %v, want %v", ms.Names(), want)
	}
}
