//go:build linux

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// openPty is a pseudo-terminal pair: the test holds the master and never
// answers on it, like a terminal that ignores colour queries.
func openPty(t *testing.T) (master, slave *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("no pty: %v", err)
	}
	t.Cleanup(func() { master.Close() })
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	n, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	slave, err = os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { slave.Close() })
	return master, slave
}

// A command an agent runs from a pane must not query the terminal: bubbletea
// v1's init asks for the background colour (OSC 11) and waits up to 5 s for a
// terminal that does not answer.
func TestACommandOnASilentTerminalSendsNoQueryAndReturnsAtOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "herdr-docket")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	for _, args := range [][]string{{"version"}, {"task", "note", "TASK-1", "a note"}} {
		t.Run(args[0], func(t *testing.T) {
			master, slave := openPty(t)
			cmd := exec.Command(bin, args...)
			cmd.Env = append(os.Environ(), "TERM=xterm-256color", "PATH="+dir, "HERDR_BIN_PATH=/nonexistent")
			cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
			var mu sync.Mutex
			var seen bytes.Buffer
			go func() {
				buf := make([]byte, 4096)
				for {
					n, err := master.Read(buf)
					mu.Lock()
					seen.Write(buf[:n])
					mu.Unlock()
					if err != nil {
						return
					}
				}
			}()
			start := time.Now()
			_ = cmd.Run() // `task note` may fail on the empty test home; only the time and the bytes count
			took := time.Since(start)
			time.Sleep(50 * time.Millisecond)
			mu.Lock()
			defer mu.Unlock()
			if bytes.Contains(seen.Bytes(), []byte("\x1b]11;?")) || bytes.Contains(seen.Bytes(), []byte("\x1b[6n")) {
				t.Errorf("queried the terminal: %q", seen.String())
			}
			if took > 500*time.Millisecond {
				t.Errorf("took %s on a silent terminal", took)
			}
		})
	}
}

// /dev/null is a character device but no terminal: `questions </dev/null`
// must take the line prompt, not open a screen nobody sees.
func TestOnlyATerminalCountsAsOne(t *testing.T) {
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	if terminal(null) {
		t.Error("/dev/null counted as a terminal")
	}
	if _, slave := openPty(t); !terminal(slave) {
		t.Error("a pty did not count as a terminal")
	}
}
