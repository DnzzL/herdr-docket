// Package changelog assembles the next release's notes from one file per
// pull request instead of one shared section.
//
// Every pull request used to append its entry to the top of `## Unreleased`
// in CHANGELOG.md. Two open pull requests therefore inserted at the same
// anchor, and git's three-way merge calls that a conflict — every time. In
// this repo that cost more than a conflict: resolving it means rebasing,
// rebasing changes the patch-id, and a changed patch-id voids the verifier's
// verdict (internal/gate). One file nobody reads during review serialised
// the whole fleet (ADR 0016).
//
// So a pull request writes `changelog.d/<name>.md` — the fleet names it
// after the task its run carries. Distinct names never conflict. The entries
// become a section only at release, when one person runs one command.
package changelog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Dir is where a pull request leaves its entry, relative to the repo root.
const Dir = "changelog.d"

// unreleased is the heading an older CHANGELOG still collects entries under.
// Nothing writes it any more; Release folds whatever it holds into the
// release being cut, so the migration costs nobody a bulk rewrite.
const unreleased = "## Unreleased"

// version is the shape .github/workflows/release.yml can find again: it
// greps "## <tag> " out of CHANGELOG.md to use as the release body, and the
// tag it looks for is the one pushed, `vX.Y.Z`.
var version = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)

// Pending is what the next release will say: every fragment in dir, ordered
// by filename, then whatever path still carries under "## Unreleased".
// Newest first, the way entries were always added to the top of the section.
// A missing dir or a missing path is not an error — both mean "nothing
// there", which is a real state before the first fragment is written.
func Pending(dir, path string) (string, error) {
	frags, err := fragments(dir)
	if err != nil {
		return "", err
	}
	legacy, err := legacySection(path)
	if err != nil {
		return "", err
	}
	if legacy != "" {
		frags = append(frags, legacy)
	}
	return strings.Join(frags, "\n\n"), nil
}

// Release folds Pending into a "## <version> — <date>" section at the top of
// path, drops the "## Unreleased" section it consumed, and deletes the
// fragment files. It returns the section it wrote.
//
// It refuses rather than guesses: a version the release workflow could not
// find again, a version already in the file, or nothing to release at all.
// The fragments are removed only once the rewritten file is on disk, so a
// failed write leaves every entry where it was.
func Release(dir, path, v string, date time.Time) (string, error) {
	if !version.MatchString(v) {
		return "", fmt.Errorf("changelog: %q is not a version the release workflow can find — it greps %q out of CHANGELOG.md, so it must read like v1.2.3", v, "## <tag> ")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("changelog: read %s: %w", path, err)
	}
	body, err := Pending(dir, path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("changelog: nothing to release — %s/ is empty and %s carries no %q section", dir, path, unreleased)
	}
	heading := fmt.Sprintf("## %s — %s", v, date.Format("2006-01-02"))
	if strings.Contains(string(raw), heading[:len("## "+v)+1]) {
		return "", fmt.Errorf("changelog: %s already has a %s section", path, v)
	}

	section := heading + "\n\n" + body + "\n"
	if err := os.WriteFile(path, []byte(replaceUnreleased(string(raw), section)), 0o644); err != nil {
		return "", fmt.Errorf("changelog: write %s: %w", path, err)
	}
	names, err := fragmentFiles(dir)
	if err != nil {
		return "", err
	}
	for _, f := range names {
		if err := os.Remove(f); err != nil {
			return "", fmt.Errorf("changelog: %s is written but %s is still there: %w", path, f, err)
		}
	}
	return section, nil
}

// replaceUnreleased puts section where the "## Unreleased" section was, or
// above the newest release when there is none. The preamble above the first
// heading — what the file is and how to read the dates — always stays put.
func replaceUnreleased(raw, section string) string {
	lines := strings.Split(raw, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == unreleased {
			start = i
			break
		}
		if strings.HasPrefix(l, "## ") {
			// No Unreleased section: the new one goes above this release.
			return splice(lines[:i], section, lines[i:])
		}
	}
	if start < 0 {
		// No headings at all: the file is only a preamble.
		return splice(lines, section, nil)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	return splice(lines[:start], section, lines[end:])
}

// splice puts section between prefix and suffix with exactly one blank line
// on each side, whatever blank lines the file already had there. Markdown
// needs the break before a heading, and the preamble above the first
// heading — what the file is and how to read the dates — always stays put.
func splice(prefix []string, section string, suffix []string) string {
	out := strings.TrimRight(strings.Join(prefix, "\n"), "\n")
	out += "\n\n" + strings.TrimRight(section, "\n") + "\n"
	if tail := strings.TrimLeft(strings.Join(suffix, "\n"), "\n"); tail != "" {
		out += "\n" + tail
	}
	return out
}

// legacySection is the body under "## Unreleased", if an older CHANGELOG
// still has one.
func legacySection(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("changelog: read %s: %w", path, err)
	}
	lines := strings.Split(string(raw), "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == unreleased {
			start = i + 1
			continue
		}
		if start >= 0 && strings.HasPrefix(l, "## ") {
			return strings.TrimSpace(strings.Join(lines[start:i], "\n")), nil
		}
	}
	if start < 0 {
		return "", nil
	}
	return strings.TrimSpace(strings.Join(lines[start:], "\n")), nil
}

// fragments is every fragment's text, ordered by filename.
func fragments(dir string) ([]string, error) {
	files, err := fragmentFiles(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("changelog: read %s: %w", f, err)
		}
		if text := strings.TrimSpace(string(raw)); text != "" {
			out = append(out, text)
		}
	}
	return out, nil
}

// fragmentFiles is every .md under dir, ordered by filename. A dir that is
// not there yet holds nothing; that is not an error.
func fragmentFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("changelog: read %s: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		// README.md documents the directory for whoever opens it on the
		// forge; it is the one .md here that is not somebody's entry.
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out, nil
}
