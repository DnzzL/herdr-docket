package fleet

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/work/backlogmd"
	"github.com/DnzzL/herdr-fleet/internal/work/basecamp"
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
	t.Setenv("HERDR_FLEET_BASECAMP_CLIENT_ID", "")
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
