package backlogmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// defaultBranchRef is the symbolic ref git keeps the remote's default branch
// behind. `git clone` writes it at clone time; `git remote set-head origin
// --auto` refreshes it, and a repo cloned before its default branch was
// renamed can carry a stale one until that is run. It is still the one
// answer git maintains in the repo itself, and the alternative — asking the
// remote over the network — is a round trip a scheduler has no business
// making. Reading a ref is cheaper than a fork of nothing.
const defaultBranchRef = "refs/remotes/origin/HEAD"

// DefaultBranch answers which ref this project's worktree runs branch from:
// the remote's default branch, read from dir, once per answer. A repo with no
// remote (or a detached clone) says so rather than answering with a branch
// that exists only locally — a worktree built on it forks a name the human
// may never have heard of, and the checkout's own HEAD is the honest fallback
// the caller applies.
func (c *cli) DefaultBranch(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("no repo behind this queue")
	}
	out, err := exec.Command("git", "-C", dir, "symbolic-ref", "--short", defaultBranchRef).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && strings.Contains(string(ee.Stderr), "not a symbolic ref") {
			return "", fmt.Errorf("%s has no remote default branch to name", dir)
		}
		return "", fmt.Errorf("git -C %s symbolic-ref: %w", dir, err)
	}
	if ref := strings.TrimSpace(string(out)); ref != "" {
		return ref, nil
	}
	return "", fmt.Errorf("%s has no remote default branch to name", dir)
}

// The Backlog.md lifecycle the fleet uses, in board order. `fleet init` writes
// this list into the project config — the CLI rejects any status outside it —
// so every status the adapter reads or writes has to be in here.
const (
	statusToDo       = "To Do"
	statusInProgress = "In Progress"
	statusBlocked    = "Blocked"
	statusFailed     = "Failed"
	statusDone       = "Done"
)

// Statuses is that lifecycle as a list, for whoever has to write it down.
var Statuses = DefaultVocabulary().List()

// task is one row of `task list --json` — enough to pick work and draw a board.
type task struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	Priority  string   `json:"priority"`
	Assignees []string `json:"assignees"`
	Labels    []string `json:"labels"`
	Ordinal   float64  `json:"ordinal"`
	CreatedAt string   `json:"createdAt"`
}

// criterion is one acceptance criterion of a task.
type criterion struct {
	Index   int    `json:"index"`
	Text    string `json:"text"`
	Checked bool   `json:"checked"`
}

// view is the full task, as `task view --json` reports it.
type view struct {
	task
	Description         string      `json:"description"`
	AcceptanceCriteria  []criterion `json:"acceptanceCriteria"`
	ImplementationNotes string      `json:"implementationNotes"`
}

// cli drives the Backlog.md CLI against one project directory. It never edits
// the markdown files itself: the CLI is the schema.
type cli struct {
	dir string
	// run is the exec seam; nil means the real CLI.
	run func(args ...string) ([]byte, error)
}

func newCLI(dir string) *cli { return &cli{dir: dir} }

func (c *cli) exec(args ...string) ([]byte, error) {
	if c.run != nil {
		return c.run(args...)
	}
	cmd := exec.Command("backlog", args...)
	cmd.Env = append(os.Environ(), "BACKLOG_CWD="+c.dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("backlog %s: %s", strings.Join(args[:min(2, len(args))], " "), msg)
	}
	return out, nil
}

// List returns every task in the project.
func (c *cli) List() ([]task, error) {
	out, err := c.exec("task", "list", "--json")
	if err != nil {
		return nil, err
	}
	var res struct {
		Tasks []task `json:"tasks"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, fmt.Errorf("task list: decode: %w", err)
	}
	return res.Tasks, nil
}

// View returns one task in full.
func (c *cli) View(id string) (view, error) {
	out, err := c.exec("task", "view", id, "--json")
	if err != nil {
		return view{}, err
	}
	var res struct {
		Task view `json:"task"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return view{}, fmt.Errorf("task view %s: decode: %w", id, err)
	}
	return res.Task, nil
}

// SetStatus moves a task. The CLI validates against the configured statuses.
func (c *cli) SetStatus(id, status string) error {
	_, err := c.exec("task", "edit", id, "-s", status, "--plain")
	return err
}

// SetAssignee replaces the task's assignees with one agent. The CLI's -a
// replaces rather than appends, which is the fleet's model: one task is one
// agent's to run.
func (c *cli) SetAssignee(id, agent string) error {
	_, err := c.exec("task", "edit", id, "-a", agent, "--plain")
	return err
}

// AppendNote adds to the task's implementation notes without replacing them.
func (c *cli) AppendNote(id, note string) error {
	_, err := c.exec("task", "edit", id, "--append-notes", note, "--plain")
	return err
}

// Create adds a task and returns its id. status starts the new task in a
// specific status — the word the project's own mapping calls To Do, which is
// how the fleet files work it creates to be picked up. Empty means the
// backend's own default, which is what a plain create has always done. The
// CLI names the new task on the plain output's header line; an id we cannot
// read is not an error — the task exists either way, and the caller can list.
func (c *cli) Create(title, description, assignee, status string) (string, error) {
	args := []string{"task", "create", title, "--plain"}
	if description != "" {
		args = append(args, "-d", description)
	}
	if assignee != "" {
		args = append(args, "-a", assignee)
	}
	if status != "" {
		args = append(args, "-s", status)
	}
	out, err := c.exec(args...)
	if err != nil {
		return "", err
	}
	return createdID(out), nil
}

// createdID reads the id out of `task create --plain`, whose first task line
// is "Task <id> - <title>".
var createdIDLine = regexp.MustCompile(`(?m)^Task\s+(\S+)\s+-`)

func createdID(out []byte) string {
	if m := createdIDLine.FindSubmatch(out); m != nil {
		return string(m[1])
	}
	return ""
}
