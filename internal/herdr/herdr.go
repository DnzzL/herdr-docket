// Package herdr is a thin client over the herdr CLI (which itself fronts the
// socket API). Where that binary lives is hostpath's problem.
//
// The calls are methods on Client rather than package functions so a caller
// that wants a narrower interface can embed it and get the whole set, instead
// of hand-forwarding a dozen one-line wrappers.
package herdr

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DnzzL/herdr-docket/internal/hostpath"
)

// Client talks to the herdr CLI. It holds nothing: the zero value is ready.
type Client struct{}

// run executes a herdr subcommand and decodes the socket-API JSON envelope
// ({"id": ..., "result": {...}}) into out when out is non-nil.
func run(out any, args ...string) error {
	cmd := exec.Command(hostpath.Bin(), args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return newAPIError(args, stdout.Bytes(), stderr.String(), err)
	}
	if out == nil {
		return nil
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		return fmt.Errorf("herdr %v: unexpected output %q: %w", args, stdout.String(), err)
	}
	raw := envelope.Result
	if raw == nil {
		raw = stdout.Bytes() // some commands print the result object bare
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("herdr %v: decode result: %w", args, err)
	}
	return nil
}

// APIError carries herdr's error code so callers can recover from the ones
// that are recoverable, and so logs get one readable line instead of a JSON
// payload.
type APIError struct {
	Command string
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code != "" && e.Message != "" {
		return e.Command + ": " + e.Code + ": " + e.Message
	}
	if e.Code != "" {
		return e.Command + ": " + e.Code
	}
	return e.Command + ": " + e.Message
}

// HasCode reports whether err is a herdr API error with the given code.
func HasCode(err error, code string) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == code
}

// newAPIError turns a failed herdr invocation into one readable line. runErr
// is the error from cmd.Run and is the last resort: a herdr that exits non-zero
// while printing nothing at all used to produce a bare "worktree create: ",
// which says only that the run failed — not that it exited 1, was killed, or
// was never on PATH.
//
// The envelope is looked for on both streams because herdr puts it on stderr,
// not stdout. Reading only stdout left Code empty on every error the fleet has
// ever seen, which silently turned every HasCode branch into dead code: the
// prompt-stall recovery, the start retries, the vanished-agent and
// vanished-workspace paths. They failed by giving up on the first try and the
// error text still read correctly, which is why it went unnoticed.
func newAPIError(args []string, stdout []byte, stderr string, runErr error) error {
	cmd := strings.Join(args[:min(2, len(args))], " ")
	for _, out := range [][]byte{stdout, []byte(stderr)} {
		var envelope struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(out, &envelope) == nil && envelope.Error.Code != "" {
			return &APIError{Command: cmd, Code: envelope.Error.Code, Message: envelope.Error.Message}
		}
	}
	msg := strings.TrimSpace(stderr)
	if msg == "" {
		msg = strings.TrimSpace(string(stdout))
	}
	if msg == "" && runErr != nil {
		msg = runErr.Error()
	}
	return &APIError{Command: cmd, Message: msg}
}

// createResult matches both worktree_created and workspace_created payloads:
// the new workspace plus its initial pane, ready for `agent start`.
type createResult struct {
	Workspace struct {
		WorkspaceID string `json:"workspace_id"`
	} `json:"workspace"`
	RootPane struct {
		PaneID string `json:"pane_id"`
	} `json:"root_pane"`
}

func (r createResult) ids(what string) (string, string, error) {
	if r.Workspace.WorkspaceID == "" || r.RootPane.PaneID == "" {
		return "", "", fmt.Errorf("%s returned no workspace/pane id", what)
	}
	return r.Workspace.WorkspaceID, r.RootPane.PaneID, nil
}

// WorktreeCreate provisions a fresh git worktree workspace off repo and
// returns its workspace and root pane IDs. Base names the ref the worktree
// branches from; empty inherits whatever the checkout has checked out, and
// then no --base is sent so nothing new is asked of herdr.
func (Client) WorktreeCreate(repo, branch, base, label string) (workspaceID, paneID string, err error) {
	args := []string{"worktree", "create",
		"--cwd", repo, "--branch", branch}
	if base != "" {
		args = append(args, "--base", base)
	}
	args = append(args, "--label", label, "--no-focus")
	var res createResult
	err = run(&res, args...)
	if err != nil {
		return "", "", err
	}
	return res.ids("worktree create")
}

// WorkspaceCreate opens a workspace directly on a directory (root mode).
func (Client) WorkspaceCreate(cwd, label string) (workspaceID, paneID string, err error) {
	var res createResult
	err = run(&res, "workspace", "create", "--cwd", cwd, "--label", label, "--no-focus")
	if err != nil {
		return "", "", err
	}
	return res.ids("workspace create")
}

// Worktree is one checkout backing a workspace. OpenWorkspaceID is empty once
// the workspace has been closed, which is the only durable signal Herdr keeps
// about whether anyone came back to look at a run.
type Worktree struct {
	Branch           string `json:"branch"`
	Path             string `json:"path"`
	OpenWorkspaceID  string `json:"open_workspace_id"`
	IsLinkedWorktree bool   `json:"is_linked_worktree"`
}

// WorktreeList returns every worktree Herdr knows about for repo, including
// the source checkout itself.
func (Client) WorktreeList(repo string) ([]Worktree, error) {
	var res struct {
		Worktrees []Worktree `json:"worktrees"`
	}
	if err := run(&res, "worktree", "list", "--cwd", repo); err != nil {
		return nil, err
	}
	return res.Worktrees, nil
}

// WorkspaceClose closes a workspace. Closing one that is already gone is not
// a failure — the point was for it not to exist — but it is still a distinct
// answer, and the caller is the one that knows whether it cares. Focus reports
// it as ErrGone; this used to swallow it, and the pane, which is the caller
// that cares most, could not then tell "I closed it" from "there was nothing
// left to close".
func (c Client) WorkspaceClose(workspaceID string) error {
	err := run(nil, "workspace", "close", workspaceID)
	if HasCode(err, CodeWorkspaceGone) {
		return ErrGone
	}
	return err
}

// AgentStart launches an interactive agent in a pane sitting at a shell
// prompt. extraArgs are forwarded to the agent executable (e.g. --mcp-config).
func (Client) AgentStart(name, kind, paneID string, extraArgs []string) error {
	args := []string{"agent", "start", name, "--kind", kind, "--pane", paneID}
	if len(extraArgs) > 0 {
		args = append(args, "--")
		args = append(args, extraArgs...)
	}
	return run(nil, args...)
}

// CodePaneBusy is herdr's refusal to start an agent in a pane that is not
// sitting at a shell prompt. It means the workspace exists but its shell has
// not spawned yet, so it is a wait-and-retry rather than a dead run: a
// sleep-delayed worktree create can return minutes before the pane is usable.
const CodePaneBusy = "agent_pane_busy"

// CodeAgentNotReady is herdr's answer when the agent process exists in the
// pane but is still starting up — a trust dialog, MCP servers connecting — and
// cannot take a prompt yet. The start succeeded; readiness is what's pending.
const CodeAgentNotReady = "agent_not_ready"

// CodeAgentGone means there is no agent in the target pane any more — the
// workspace was closed, or the agent exited on its own. herdr only reports it
// when the call it was given returns, so a wait handed the run's whole timeout
// sits on a dead pane for that long before saying so.
const CodeAgentGone = "agent_not_running"

// CodeWorkspaceGone is herdr's answer when the workspace ID no longer names
// anything — the expected result of asking about a run somebody has reviewed
// and closed.
const CodeWorkspaceGone = "workspace_not_found"

// CodeStalled is herdr's verdict when a submitted prompt produces no visible
// state change within 5 seconds. It does not mean the prompt was lost — an
// agent still loading its MCP servers takes longer than that to react.
const CodeStalled = "agent_prompt_stalled"

// AgentSubmit types a prompt into the agent and asks herdr to confirm the
// agent reacted. A CodeStalled error is inconclusive; callers should check the
// status before giving up.
func (Client) AgentSubmit(target, text string) error {
	return run(nil, "agent", "prompt", target, text, "--wait", "--until", "working",
		"--timeout", "30000")
}

// AgentStatus reports the agent's current state: idle, working, blocked…
func (Client) AgentStatus(target string) (string, error) {
	var res struct {
		Agent struct {
			AgentStatus string `json:"agent_status"`
		} `json:"agent"`
	}
	if err := run(&res, "agent", "get", target); err != nil {
		return "", err
	}
	return res.Agent.AgentStatus, nil
}

// AgentSubmitPending presses Enter on the pane, submitting anything already
// sitting in the composer. Harmless when the composer is empty.
func (Client) AgentSubmitPending(paneID string) error {
	return run(nil, "pane", "send-keys", paneID, "enter")
}

// AgentWait blocks until the agent settles (idle, done or blocked).
func (Client) AgentWait(target string, timeout time.Duration) error {
	return run(nil, "agent", "wait", target,
		"--timeout", fmt.Sprintf("%d", timeout.Milliseconds()))
}

// Notify raises Herdr's own desktop notification. It is how a run that ended
// failed reaches a human the daemon could not otherwise address: the fleet
// reports through the host rather than owning a channel of its own. Reports
// only — nothing here decides anything.
func (Client) Notify(title, body string) error {
	args := []string{"notification", "show", title}
	if body != "" {
		args = append(args, "--body", body)
	}
	return run(nil, args...)
}

// ErrGone means the run's workspace no longer exists — the expected outcome
// once you've reviewed and closed it, not a failure worth a stack trace. It is
// a fact about the workspace, not a verdict about the run: a caller that only
// wanted the workspace gone treats it as success, and one that was reporting
// on the workspace says what it found.
var ErrGone = errors.New("workspace already closed")

// Focus brings a run's workspace to the front, then its agent pane when one
// is known — the "jump to what this automation did" move.
func (Client) Focus(workspaceID, paneID string) error {
	if workspaceID != "" {
		if err := run(nil, "workspace", "focus", workspaceID); err != nil {
			if HasCode(err, CodeWorkspaceGone) {
				return ErrGone
			}
			return err
		}
	}
	if paneID != "" {
		// Best-effort: the pane may be gone while the workspace lives on.
		_ = run(nil, "agent", "focus", paneID)
	}
	return nil
}

// PaneRun executes a shell command in a pane (used to delegate to hwf).
func (Client) PaneRun(paneID string, command ...string) error {
	return run(nil, append([]string{"pane", "run", paneID}, command...)...)
}

// PaneRead returns the pane's recent terminal output. A pane is the only
// channel a delegated command has, so this is how its result gets read back.
// Unlike the rest of the API this one prints the screen, not a JSON envelope.
func (Client) PaneRead(paneID string, lines int) (string, error) {
	cmd := exec.Command(hostpath.Bin(), "pane", "read", paneID,
		"--source", "recent", "--lines", fmt.Sprintf("%d", lines), "--format", "text")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", newAPIError([]string{"pane", "read"}, stdout.Bytes(), stderr.String(), err)
	}
	return stdout.String(), nil
}

// tabResult is what `tab create` answers with: the pane to start an agent in,
// and the tab that holds it — which is what has to be closed again, because a
// tab-run does not own the workspace it is sitting in.
type tabResult struct {
	Tab struct {
		TabID string `json:"tab_id"`
	} `json:"tab"`
	RootPane struct {
		PaneID string `json:"pane_id"`
	} `json:"root_pane"`
}

// TabCreate opens a tab on cwd inside an existing workspace and returns the
// pane to work in and the tab to close afterwards.
func (Client) TabCreate(workspaceID, cwd, label string) (paneID, tabID string, err error) {
	var res tabResult
	err = run(&res, "tab", "create",
		"--workspace", workspaceID, "--cwd", cwd, "--label", label, "--no-focus")
	if err != nil {
		return "", "", err
	}
	if res.RootPane.PaneID == "" || res.Tab.TabID == "" {
		return "", "", fmt.Errorf("tab create returned no pane/tab id")
	}
	return res.RootPane.PaneID, res.Tab.TabID, nil
}

// TabClose closes one tab. It is how a run that borrowed a workspace gives it
// back: closing the workspace would take every sibling run — and the human's
// own tab — with it.
func (c Client) TabClose(tabID string) error { return run(nil, "tab", "close", tabID) }

// workspaceList is the shape of `workspace list` this package needs: which
// repository each workspace is a checkout of, and whether it is the primary
// one or a linked worktree of it.
type workspaceList struct {
	Workspaces []struct {
		WorkspaceID string `json:"workspace_id"`
		Worktree    *struct {
			RepoRoot         string `json:"repo_root"`
			IsLinkedWorktree bool   `json:"is_linked_worktree"`
		} `json:"worktree"`
	} `json:"workspaces"`
}

// PrimaryWorkspace is the workspace a repository is already open in: the one
// herdr calls primary, as opposed to the linked worktree workspaces grouped
// under it. Empty means the repo has no workspace open, which is an answer
// and not a failure — the caller opens its own, as it always did.
//
// Derived rather than configured on purpose: a workspace id written into a
// config file is a reference that rots the first time somebody closes it.
func (Client) PrimaryWorkspace(repoRoot string) (string, error) {
	var res workspaceList
	if err := run(&res, "workspace", "list"); err != nil {
		return "", err
	}
	for _, w := range res.Workspaces {
		if w.Worktree == nil || w.Worktree.IsLinkedWorktree {
			continue
		}
		if filepath.Clean(w.Worktree.RepoRoot) == filepath.Clean(repoRoot) {
			return w.WorkspaceID, nil
		}
	}
	return "", nil
}
