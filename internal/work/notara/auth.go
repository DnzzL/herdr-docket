package notara

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"

	credstore "github.com/DnzzL/herdr-docket/internal/credentials"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// Login checks a key against the configured database, stores it when it was
// handed over, and provisions the database with the fleet's words: every
// status word the fleet writes and every agent it routes to becomes an
// option of its select. Notara refuses a select value that is not an option
// (422), so a fleet that never signed in would fail its first close. Once,
// at sign-in, with the whole option set in hand — never lazily mid-run,
// racing a human editing the same field.
func Login(c Config, vocab work.Vocabulary, agents []string, handed string, out io.Writer) error {
	return login(c, vocab, agents, handed, credstore.Default(), out)
}

func login(c Config, vocab work.Vocabulary, agents []string, handed string, store credstore.File, out io.Writer) error {
	s, err := New(c, vocab, "")
	if err != nil {
		return err
	}
	handed = strings.TrimSpace(handed)
	if handed != "" {
		s.api.token = func() (string, error) { return handed, nil }
	} else {
		s.api.token = (&tokenSource{store: store, env: os.Getenv}).access
	}

	// The fields are the check: a key Notara refuses is never written down,
	// where every later tick would find it and report Notara's complaint.
	var fields []field
	if err := s.api.do(http.MethodGet, "/fields", nil, &fields); err != nil {
		return err
	}
	fmt.Fprintf(out, "Signed in to Notara at %s, database %s\n", s.cfg.URL, s.cfg.Database)
	if handed != "" {
		if err := store.Save(section, stored{Token: handed}); err != nil {
			return err
		}
		fmt.Fprintf(out, "Key stored in %s\n", store.Path)
	}

	return errors.Join(
		s.provision(fields, s.cfg.statusField(), vocab.List(), out),
		s.provision(fields, s.cfg.agentField(), agents, out),
	)
}

// provision adds the words a select is missing, after the ones it has.
func (s *Source) provision(fields []field, name string, want []string, out io.Writer) error {
	i := slices.IndexFunc(fields, func(f field) bool { return f.Name == name })
	if i < 0 {
		return fmt.Errorf("notara: the database has no %q field — create it as a select, or name yours in source.notara", name)
	}
	f := fields[i]
	if f.Type != "select" {
		return fmt.Errorf("notara: field %q is a %s; the fleet writes one word into it, so it must be a select", name, f.Type)
	}
	options := slices.Clone(f.Options)
	var added []string
	for _, w := range want {
		if w != "" && !slices.Contains(options, w) {
			options = append(options, w)
			added = append(added, w)
		}
	}
	if len(added) == 0 {
		return nil
	}
	if err := s.api.do(http.MethodPatch, "/fields/"+f.ID, map[string]any{"options": options}, nil); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s: added %s\n", name, strings.Join(added, ", "))
	return nil
}

type field struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Options []string `json:"options"`
}
