// Package backlog is a thin client over the Backlog.md CLI, pointed at the
// fleet's platform backlog via BACKLOG_CWD. It never edits the markdown files
// itself: the CLI is the schema.
package backlog

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// The fleet's task lifecycle. `fleet init` writes these into the Backlog.md
// project config; the CLI rejects any status outside the configured list.
const (
	StatusToDo       = "To Do"
	StatusInProgress = "In Progress"
	StatusBlocked    = "Blocked"
	StatusFailed     = "Failed"
	StatusDone       = "Done"
)

// Statuses is the full lifecycle, in board order.
var Statuses = []string{StatusToDo, StatusInProgress, StatusBlocked, StatusFailed, StatusDone}

// Task is one row of `task list --json` — enough to pick work and draw a board.
type Task struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	Priority  string   `json:"priority"`
	Assignees []string `json:"assignees"`
	Labels    []string `json:"labels"`
	Ordinal   float64  `json:"ordinal"`
	CreatedAt string   `json:"createdAt"`
}

// Criterion is one acceptance criterion of a task.
type Criterion struct {
	Index   int    `json:"index"`
	Text    string `json:"text"`
	Checked bool   `json:"checked"`
}

// View is the full task, as `task view --json` reports it.
type View struct {
	Task
	Description         string      `json:"description"`
	AcceptanceCriteria  []Criterion `json:"acceptanceCriteria"`
	ImplementationNotes string      `json:"implementationNotes"`
}

// Client calls the backlog CLI against one project directory.
type Client struct {
	Dir string
	// run is the exec seam; nil means the real CLI.
	run func(args ...string) ([]byte, error)
}

// New returns a client on the Backlog.md project at dir.
func New(dir string) *Client { return &Client{Dir: dir} }

func (c *Client) exec(args ...string) ([]byte, error) {
	if c.run != nil {
		return c.run(args...)
	}
	cmd := exec.Command("backlog", args...)
	cmd.Env = append(os.Environ(), "BACKLOG_CWD="+c.Dir)
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
func (c *Client) List() ([]Task, error) {
	out, err := c.exec("task", "list", "--json")
	if err != nil {
		return nil, err
	}
	var res struct {
		Tasks []Task `json:"tasks"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, fmt.Errorf("task list: decode: %w", err)
	}
	return res.Tasks, nil
}

// View returns one task in full.
func (c *Client) View(id string) (View, error) {
	out, err := c.exec("task", "view", id, "--json")
	if err != nil {
		return View{}, err
	}
	var res struct {
		Task View `json:"task"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return View{}, fmt.Errorf("task view %s: decode: %w", id, err)
	}
	return res.Task, nil
}

// SetStatus moves a task. The CLI validates against the configured statuses.
func (c *Client) SetStatus(id, status string) error {
	_, err := c.exec("task", "edit", id, "-s", status, "--plain")
	return err
}

// AppendNote adds to the task's implementation notes without replacing them.
func (c *Client) AppendNote(id, note string) error {
	_, err := c.exec("task", "edit", id, "--append-notes", note, "--plain")
	return err
}

// Create adds a task and returns nothing — the daemon will find it on the
// next tick like any other. Used by the board's add flow.
func (c *Client) Create(title, description, assignee string) error {
	args := []string{"task", "create", title, "--plain"}
	if description != "" {
		args = append(args, "-d", description)
	}
	if assignee != "" {
		args = append(args, "-a", assignee)
	}
	_, err := c.exec(args...)
	return err
}
