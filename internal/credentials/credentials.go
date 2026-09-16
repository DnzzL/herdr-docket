// Package credentials owns credentials.yaml: one file beside fleet.yaml,
// holding one section per backend that needs a secret.
//
// It exists because that file has several writers. A backend that stores a
// token writes its own section and must leave every other section — including
// one written by a version of the fleet it has never heard of — exactly as it
// found it. Each adapter marshalling the whole file itself would make the
// last writer the only backend with credentials.
//
// The file is a secret: written 0600, and chmod'd on every save, because
// WriteFile's mode only applies when it creates the file.
package credentials

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/DnzzL/herdr-docket/internal/hostpath"
)

// File is a credentials file at a path. The path is a field rather than a
// package-level decision so tests can put one in a temp dir.
type File struct {
	Path string
}

// Default is the fleet's credentials.yaml: beside fleet.yaml in the config
// dir, and never inside it — fleet.yaml is the file people paste into a bug
// report.
func Default() File {
	return File{Path: filepath.Join(hostpath.ConfigDir(), "credentials.yaml")}
}

// Load decodes the named section into v. A file that is not there, or a
// section that is not in it, leaves v as it was and is not an error: never
// having logged in is a state a fleet runs in, not a failure.
func (f File) Load(section string, v any) error {
	sections, err := f.sections()
	if err != nil {
		return err
	}
	node, ok := sections[section]
	if !ok {
		return nil
	}
	return node.Decode(v)
}

// Save encodes v as the named section and writes the whole file back, with
// every other section untouched. The read-modify-write is the point: another
// backend's secret is not this backend's to drop.
func (f File) Save(section string, v any) error {
	sections, err := f.sections()
	if err != nil {
		return err
	}
	var node yaml.Node
	if err := node.Encode(v); err != nil {
		return err
	}
	sections[section] = node

	raw, err := yaml.Marshal(sections)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(f.Path, raw, 0o600); err != nil {
		return err
	}
	return os.Chmod(f.Path, 0o600)
}

// sections reads the file as one node per section, so a section this build
// does not know the shape of still survives being written back. An empty or
// missing file is an empty map.
func (f File) sections() (map[string]yaml.Node, error) {
	raw, err := os.ReadFile(f.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]yaml.Node{}, nil
		}
		return nil, err
	}
	sections := map[string]yaml.Node{}
	// An empty file is not a document, and yaml reports that as an error
	// rather than as nothing; a file that is there but blank is the same
	// state as one that is not.
	if len(raw) == 0 {
		return sections, nil
	}
	if err := yaml.Unmarshal(raw, &sections); err != nil {
		return nil, err
	}
	if sections == nil {
		sections = map[string]yaml.Node{}
	}
	return sections, nil
}
