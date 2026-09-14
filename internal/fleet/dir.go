package fleet

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/DnzzL/herdr-docket/internal/hostpath"
)

// Settings is fleet.yaml in the plugin's config dir. Optional: the zero
// value — everything under ~/fleet, no default agent — is a working setup.
type Settings struct {
	// Dir is where the platform lives: the Backlog.md project and agents/.
	Dir string `yaml:"dir"`
	// DefaultAgent picks up tasks that have no assignee. Empty means
	// unassigned tasks are simply not the fleet's business.
	DefaultAgent string `yaml:"default_agent"`
	// Source is where the queue lives. Absent, it is the Backlog.md project
	// in Dir.
	Source SourceConfig `yaml:"source"`
	// Sources is several queues at once, name → the same block Source takes.
	// Each name becomes the prefix on its tasks' ids. Setting both Source and
	// Sources is an error: one fleet works one arrangement of queues.
	Sources map[string]SourceConfig `yaml:"sources"`
}

// LoadSettings reads fleet.yaml, fills defaults, expands ~. A missing file is
// the default setup; a broken one is an error worth stopping on, because a
// daemon watching the wrong directory looks exactly like a daemon that works.
func LoadSettings() (Settings, error) {
	var s Settings
	raw, err := os.ReadFile(filepath.Join(hostpath.ConfigDir(), "fleet.yaml"))
	if err == nil {
		if err := yaml.Unmarshal(raw, &s); err != nil {
			return Settings{}, err
		}
		// Presence, not emptiness: a source: block that happens to be blank
		// still contradicts sources:. Two arrangements is a decision, and the
		// fleet will not pick one for you.
		var keys map[string]any
		if err := yaml.Unmarshal(raw, &keys); err != nil {
			return Settings{}, err
		}
		_, hasSource := keys["source"]
		_, hasSources := keys["sources"]
		if hasSource && hasSources {
			return Settings{}, fmt.Errorf("fleet.yaml sets both source: and sources: — use one arrangement of queues, not both")
		}
	} else if !os.IsNotExist(err) {
		return Settings{}, err
	}
	if s.Dir == "" {
		home, _ := os.UserHomeDir()
		s.Dir = filepath.Join(home, "fleet")
	}
	s.Dir = expandHome(s.Dir)
	return s, nil
}
