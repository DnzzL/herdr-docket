package daemon

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Tail reads the daemon log's most recent lines, in file order — the tail a
// human asks for when the fleet misbehaved and they want one screen of why.
// It is a read of a file this fleet already wrote (the plugin's startup hook
// redirects the daemon's stderr there), not a second log of its own.
func Tail(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("no daemon log at %s — is the plugin's daemon running?", path)
		}
		return nil, err
	}
	defer f.Close()

	// Only the tail is held: a log nothing here ever truncates is read to
	// its end, but a month of ticks must not cost a month of memory.
	ring := make([]string, 0, n)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if len(ring) == n {
			ring = ring[1:]
		}
		ring = append(ring, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ring, nil
}
