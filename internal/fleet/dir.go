package fleet

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/DnzzL/herdr-fleet/internal/hostpath"
)

// Settings is fleet.yaml in the plugin's config dir. Optional: the zero
// value — everything under ~/fleet, no default agent — is a working setup.
type Settings struct {
	// Dir is where the platform lives: the Backlog.md project and agents/.
	Dir string `yaml:"dir"`
	// DefaultAgent picks up tasks that have no assignee. Empty means
	// unassigned tasks are simply not the fleet's business.
	DefaultAgent string `yaml:"default_agent"`
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
