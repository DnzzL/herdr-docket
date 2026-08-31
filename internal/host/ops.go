package host

import (
	"time"

	"github.com/DnzzL/herdr-fleet/internal/herdr"
)

// ops is this package's internal seam: the individual Herdr calls, one method
// each. It is unexported on purpose — callers of Host must not have to know
// that starting an agent can answer "the pane is not a shell yet", only that
// the work either happened or didn't. The tests script it.
type ops interface {
	WorktreeCreate(repo, branch, label string) (workspaceID, paneID string, err error)
	WorkspaceCreate(cwd, label string) (workspaceID, paneID string, err error)

	AgentStart(name, kind, paneID string, extraArgs []string) error
	AgentSubmit(target, text string) error
	AgentSubmitPending(paneID string) error
	AgentStatus(target string) (string, error)
	AgentWait(target string, timeout time.Duration) error

	// HasCode reports whether err is a Herdr API error with the given code.
	// It travels with the ops so a fake can answer for its own errors.
	HasCode(err error, code string) bool
}

// herdrOps is the production ops. The Herdr calls come from the embedded
// client; error-code matching is not Herdr's business, so it lives here.
type herdrOps struct {
	herdr.Client
}

func (herdrOps) HasCode(err error, code string) bool { return herdr.HasCode(err, code) }
