package fleet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/work/backlogmd"
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

// writeFleetYAML points the config dir at a temp dir holding one fleet.yaml.
func writeFleetYAML(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fleet.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", dir)
}
