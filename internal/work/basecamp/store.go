package basecamp

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/DnzzL/herdr-fleet/internal/hostpath"
)

// credentialsFile is the shape of credentials.yaml: a section per backend, so
// one file holds the fleet's secrets without them colliding. It sits beside
// fleet.yaml and never inside it — fleet.yaml is the file people paste into a
// bug report.
type credentialsFile struct {
	Basecamp Token `yaml:"basecamp"`
}

// fileStore is where a Basecamp token lives between runs.
type fileStore struct{ path string }

func defaultStore() fileStore {
	return fileStore{path: filepath.Join(hostpath.ConfigDir(), "credentials.yaml")}
}

func (s fileStore) load() (Token, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Token{}, nil
		}
		return Token{}, err
	}
	var f credentialsFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return Token{}, err
	}
	return f.Basecamp, nil
}

// save writes the token at 0600. The chmod is not redundant: WriteFile's mode
// only applies when it creates the file, so a credentials.yaml that was once
// world-readable would stay that way.
func (s fileStore) save(t Token) error {
	raw, err := yaml.Marshal(credentialsFile{Basecamp: t})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(s.path, raw, 0o600); err != nil {
		return err
	}
	return os.Chmod(s.path, 0o600)
}
