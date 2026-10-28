package herdr

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DnzzL/herdr-docket/internal/hostpath"
	"github.com/DnzzL/herdr-docket/internal/testenv"
)

// TestMain keeps this package's tests off the owner's real home, config and
// state dirs (TASK-84), and off the owner's live Herdr session (TASK-94): the
// contract suite execs the real herdr binary, so without a session of its own
// every `go test ./...` opened and closed `docket contract` panes in the
// session a human was typing into.
//
// The environment has to be settled first — herdr derives the session socket
// from HOME/XDG_CONFIG_HOME — which is what MainWith's hook is for.
func TestMain(m *testing.M) { os.Exit(testenv.MainWith(m, startContractSession)) }

// contractSession is the throwaway herdr session every contract call names.
// Empty means the suite cannot run here: requireHerdr skips when herdr is
// absent, and fails with contractSessionErr when herdr is there but its
// session never answered.
var (
	contractSession    string
	contractSessionErr error
)

// contractSupervisor is the shell that owns the throwaway session's server.
// $1 binary, $2 session name, $3 the test binary's pid.
//
// It runs outside TestMain because the process that must clean up is the one
// that is gone when a test times out. `go test -timeout` panics the test
// binary without unwinding TestMain, so the server would outlive it — and a
// session left running on the owner's machine is the bug this whole file
// answers. The supervisor watches the test binary by pid, and on its death or
// on SIGINT from TestMain stops the server, then deletes the session (herdr
// refuses to delete a running one, and releases it a beat after the kill, so
// the delete is retried).
const contractSupervisor = `"$1" --session "$2" server >/dev/null 2>&1 &
child=$!
stop=0
trap 'stop=1' INT TERM
while [ "$stop" -eq 0 ] && kill -0 "$3" 2>/dev/null; do sleep 0.2; done
kill "$child" 2>/dev/null
wait "$child" 2>/dev/null
i=0
while [ "$i" -lt 25 ]; do
  "$1" session delete "$2" >/dev/null 2>&1 && exit 0
  i=$((i+1))
  sleep 0.2
done
exit 1
`

// startContractSession starts the suite's own herdr session and returns the
// func that tears it down. A fixed name would collide with a parallel package
// or adopt a half-dead server from an earlier run, so the test binary's pid
// names it.
func startContractSession() func() {
	bin := hostpath.Bin()
	if _, err := exec.LookPath(bin); err != nil {
		// No herdr (CI): the supervisor has nothing to run, and every
		// contract test skips through requireHerdr.
		return nil
	}
	name := fmt.Sprintf("docket-contract-%d", os.Getpid())
	cmd := exec.Command("/bin/sh", "-c", contractSupervisor, "herdr-contract",
		bin, name, strconv.Itoa(os.Getpid()))
	// The server's panes run whichever agent the owner has installed, and
	// that agent reads its own home — pi keeps its sessions under ~/.pi. The
	// tests themselves stay isolated by testenv; only the server's children
	// see the real HOME, which is what makes the busy-pane contract honest
	// and keeps session storage under the temp XDG dir (herdr prefers it).
	cmd.Env = withHome(os.Environ(), testenv.RealHome())
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		contractSessionErr = fmt.Errorf("start the throwaway herdr session %s: %w", name, err)
		return nil
	}
	c := Client{Session: name}
	deadline := time.Now().Add(15 * time.Second)
	var lastErr error
	for {
		if _, lastErr = c.PrimaryWorkspace(""); lastErr == nil {
			break
		}
		if !time.Now().Before(deadline) {
			contractSessionErr = fmt.Errorf("the throwaway herdr session %s never answered: %w", name, lastErr)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if contractSessionErr == nil {
		contractSession = name
	}
	return func() {
		// SIGINT stops the supervisor's watch even if this process lingers,
		// and it does the stop-and-delete itself before exiting.
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(20 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	}
}

// withHome replaces HOME in env, leaving the rest as Main set it.
func withHome(env []string, home string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if !strings.HasPrefix(kv, "HOME=") {
			out = append(out, kv)
		}
	}
	return append(out, "HOME="+home)
}
