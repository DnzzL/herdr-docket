// Package github adapts a GitHub Projects v2 board to work.Source: the board
// is the queue, and a task is a real issue that has been put on it.
//
// Issues rather than the board's own draft cards, because a draft card cannot
// be commented on, and work that leaves no trace is not work a fleet can hand
// on. The issue keeps its own state, its own thread and its own history; the
// board contributes what an issue has no concept of — an order, a column, and
// the field that names the agent. Its kind in fleet.yaml is `github`.
package github

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/DnzzL/herdr-docket/internal/work"
)

// Defaults for the names on the board. A board has to be told these only when
// a human has named something differently.
const (
	defaultAgentField  = "Agent"
	defaultStatusField = "Status"
	defaultInProgress  = "In Progress"
)

// verdictPrefix marks the comment that carries a task's verdict. It is spelled
// exactly as Basecamp's is: the word is read by a person as often as by the
// fleet, and two adapters disagreeing about it would be two dialects for one
// idea.
const verdictPrefix = "Verdict: "

// stateOpen is GitHub's word for an issue nobody has closed.
const stateOpen = "OPEN"

// Config is the github block of fleet.yaml. Each adapter owns the shape of its
// own configuration, so this is the one place the board's coordinates are
// named.
type Config struct {
	Owner       string `yaml:"owner"`        // the board's owner: an org or a user
	Project     int    `yaml:"project"`      // the number in .../projects/3
	Repo        string `yaml:"repo"`         // owner/name: where Create writes and what List reads
	AgentField  string `yaml:"agent_field"`  // default Agent
	StatusField string `yaml:"status_field"` // default Status
	InProgress  string `yaml:"in_progress"`  // default "In Progress"
}

func (c Config) agentField() string {
	if c.AgentField == "" {
		return defaultAgentField
	}
	return c.AgentField
}

func (c Config) statusField() string {
	if c.StatusField == "" {
		return defaultStatusField
	}
	return c.StatusField
}

func (c Config) inProgress() string {
	if c.InProgress == "" {
		return defaultInProgress
	}
	return c.InProgress
}

// repo is the repository Create writes into and List reads: the configured
// "owner/name", split once where the configuration is read rather than at every
// document that needs the two halves.
type repo struct{ owner, name string }

// parseRepo reads "owner/name". A name with a slash in it is not a repository.
func parseRepo(s string) (repo, error) {
	owner, name, ok := strings.Cut(strings.TrimSpace(s), "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return repo{}, fmt.Errorf("github: source.github.repo is %q, want owner/name", s)
	}
	return repo{owner: owner, name: name}, nil
}

// Source is the adapter. Its fields are unexported so a test can swap the API
// client for one that talks to a fake, exactly as basecamp's does.
type Source struct {
	cfg  Config
	repo repo
	api  *api
}

var (
	_ work.Source   = (*Source)(nil)
	_ work.Phaser   = (*Source)(nil)
	_ work.Assigner = (*Source)(nil)
)

// New builds a Source. It validates the configuration and talks to nothing:
// the fleet builds a Source on every tick, and a tick must not fail because
// GitHub is briefly unreachable.
func New(c Config) (*Source, error) {
	switch {
	case strings.TrimSpace(c.Owner) == "":
		return nil, fmt.Errorf("github: source.github.owner is required — the org or user that owns the board")
	case c.Project <= 0:
		return nil, fmt.Errorf("github: source.github.project is required — the number in .../projects/<n>")
	case strings.TrimSpace(c.Repo) == "":
		return nil, fmt.Errorf("github: source.github.repo is required — owner/name, the repository Create writes issues to")
	}
	r, err := parseRepo(c.Repo)
	if err != nil {
		return nil, err
	}
	return &Source{
		cfg:  c,
		repo: r,
		api: &api{
			url:    GraphQLURL,
			http:   defaultHTTP(),
			tokens: defaultTokens(),
			sleep:  time.Sleep,
			now:    time.Now,
		},
	}, nil
}

// List reads one page of the board, in the board's order, as the queue.
//
// One page and no cursor: work.Source has no next-page, so the tail of a
// hundred-item board is a limit that is declared rather than hidden. The page
// asks for nothing the queue does not read — no bodies, no comments, no field
// values beyond the two columns — because points are charged per hundred
// objects returned, and a comment stream per item would turn a five-second
// poll from about a point into about a hundred.
func (s *Source) List() ([]work.Task, error) {
	var out struct {
		Organization *boardHalf `json:"organization"`
		User         *boardHalf `json:"user"`
	}
	if err := s.api.do("Items", itemsQuery, map[string]any{
		"owner":       s.cfg.Owner,
		"project":     s.cfg.Project,
		"filter":      "repo:" + s.cfg.Repo + " is:issue",
		"agentField":  s.cfg.agentField(),
		"statusField": s.cfg.statusField(),
	}, &out); err != nil {
		return nil, err
	}
	board, err := s.project(out.Organization, out.User)
	if err != nil {
		return nil, err
	}
	items := board.Items.Nodes
	tasks := make([]work.Task, 0, len(items))
	for i, n := range items {
		if n.Content == nil || n.Content.Number == 0 {
			// A draft card, or a pull request that got past the filter. The
			// board can carry both; neither is the fleet's work.
			continue
		}
		open := n.Content.State == stateOpen
		tasks = append(tasks, work.Task{
			ID:        strconv.Itoa(n.Content.Number),
			Title:     n.Content.Title,
			Assignee:  n.Agent.Name,
			Open:      open,
			Phase:     s.phase(n.Status.Name, open),
			Ordinal:   float64(i), // the board's order is the queue's order
			CreatedAt: n.Content.CreatedAt,
		})
	}
	return tasks, nil
}

// Get reads one task in full: the body, the checklist, the thread, and where
// the issue stands on the board. This is the call a person makes, not the
// poll, and it is the only one that pays for a body.
func (s *Source) Get(id string) (work.Task, error) {
	number, err := issueNumber(id)
	if err != nil {
		return work.Task{}, err
	}
	var out struct {
		Organization *boardHalf `json:"organization"`
		User         *boardHalf `json:"user"`
		Repository   struct {
			Issue *struct {
				Number    int    `json:"number"`
				Title     string `json:"title"`
				Body      string `json:"body"`
				State     string `json:"state"`
				CreatedAt string `json:"createdAt"`
				Comments  struct {
					Nodes []comment `json:"nodes"`
				} `json:"comments"`
				ProjectItems struct {
					Nodes []itemNode `json:"nodes"`
				} `json:"projectItems"`
			} `json:"issue"`
		} `json:"repository"`
	}
	if err := s.api.do("Issue", issueQuery, s.issueVars(number), &out); err != nil {
		return work.Task{}, err
	}
	issue := out.Repository.Issue
	if issue == nil {
		return work.Task{}, fmt.Errorf("github: no issue %d in %s", number, s.cfg.Repo)
	}
	board, err := s.project(out.Organization, out.User)
	if err != nil {
		return work.Task{}, err
	}
	item, err := s.item(board, issue.ProjectItems.Nodes, number)
	if err != nil {
		return work.Task{}, err
	}

	open := issue.State == stateOpen
	thread := notes(issue.Comments.Nodes)
	task := work.Task{
		ID:        strconv.Itoa(issue.Number),
		Title:     issue.Title,
		Body:      issue.Body,
		Notes:     thread,
		Assignee:  item.Agent.Name,
		Open:      open,
		Phase:     s.phase(item.Status.Name, open),
		CreatedAt: issue.CreatedAt,
		Criteria:  criteria(issue.Body),
	}
	if !open {
		// The issue's state says the work stopped; only the thread says how it
		// went. A verdict is read back here and nowhere else, and only from a
		// closed issue: an open one still carrying "Verdict: blocked" is work
		// a human has since unblocked, and the phase must not follow the
		// stale word.
		if v := verdictOf(thread); v != "" {
			task.Verdict = v
			task.Phase = v.Label()
		}
	}
	return task, nil
}

// Create writes a new issue and puts it on the board, as one task.
//
// The routing key is checked before anything is written. An agent that is not
// an option on the board is a mistake in configuration, and an issue created
// to discover it would be an issue nobody asked for. The issue and its place
// on the board are one mutation, so the create cannot half-happen either.
func (s *Source) Create(title, body, agent string) (string, error) {
	t, err := s.createTarget()
	if err != nil {
		return "", err
	}
	var routed *option
	if agent != "" {
		field := t.project.field(s.cfg.agentField())
		if field == nil {
			return "", fmt.Errorf("github: the board has no %q field — run `herdr-docket auth github` to create it", s.cfg.agentField())
		}
		if routed = field.option(agent); routed == nil {
			return "", fmt.Errorf("github: agent %q is not an option of the board's %q field — run `herdr-docket auth github` to add it", agent, s.cfg.agentField())
		}
	}

	var out struct {
		CreateIssue struct {
			Issue *struct {
				Number       int `json:"number"`
				ProjectItems struct {
					Nodes []itemNode `json:"nodes"`
				} `json:"projectItems"`
			} `json:"issue"`
		} `json:"createIssue"`
	}
	if err := s.api.do("CreateIssue", createIssueMutation, map[string]any{
		"repo":    t.repoID,
		"project": t.project.ID,
		"title":   title,
		"body":    body,
	}, &out); err != nil {
		return "", err
	}
	issue := out.CreateIssue.Issue
	if issue == nil || issue.Number == 0 {
		return "", fmt.Errorf("github: creating %q answered no issue", title)
	}
	id := strconv.Itoa(issue.Number)
	if routed == nil {
		return id, nil
	}

	item, err := s.item(&t.project, issue.ProjectItems.Nodes, issue.Number)
	if err != nil {
		// The issue is written but the board has not answered with its item.
		// Say which issue, so the work can be found and routed by hand.
		return "", fmt.Errorf("github: created issue #%s but could not find it on %s: %w", id, s.boardName(), err)
	}
	field := t.project.field(s.cfg.agentField())
	if err := s.setField(&t.project, item.ID, field, routed); err != nil {
		return "", fmt.Errorf("github: created issue #%s but could not route it to %q: %w", id, agent, err)
	}
	return id, nil
}

// Comment appends a note to the issue's own thread.
func (s *Source) Comment(id, text string) error {
	t, err := s.locate(id)
	if err != nil {
		return err
	}
	var out struct {
		AddComment struct {
			CommentEdge struct {
				Node struct {
					ID string `json:"id"`
				} `json:"node"`
			} `json:"commentEdge"`
		} `json:"addComment"`
	}
	return s.api.do("AddComment", addCommentMutation, map[string]any{
		"subject": t.issueID,
		"body":    text,
	}, &out)
}

// Close ends the work: the issue is closed with the reason the verdict
// implies, and the verdict is written to the thread.
//
// The state reason is for people passing through GitHub. COMPLETED says the
// work was done and everything else says it was not, because GitHub's
// vocabulary cannot tell failed from blocked — that distinction lives in the
// comment, which is the verdict's real home.
func (s *Source) Close(id string, v work.Verdict) error {
	if !v.Known() {
		return fmt.Errorf("github: refusing to close %s with %q, which is not a verdict", id, string(v))
	}
	t, err := s.locate(id)
	if err != nil {
		return err
	}
	reason := "NOT_PLANNED"
	if v == work.Done {
		reason = "COMPLETED"
	}
	var out struct {
		CloseIssue struct {
			Issue struct {
				State string `json:"state"`
			} `json:"issue"`
		} `json:"closeIssue"`
	}
	return s.api.do("Close", closeMutation, map[string]any{
		"issue":  t.issueID,
		"reason": reason,
		"body":   verdictPrefix + string(v),
	}, &out)
}

// Assign moves the task's Agent option, which is how a run is handed on.
//
// The option must already exist: a write that cleared the field instead would
// take the task off every agent's board at once, and look like it worked.
func (s *Source) Assign(id, agent string) error {
	t, err := s.locate(id)
	if err != nil {
		return err
	}
	field := t.project.field(s.cfg.agentField())
	if field == nil {
		return fmt.Errorf("github: the board has no %q field — run `herdr-docket auth github` to create it", s.cfg.agentField())
	}
	routed := field.option(agent)
	if routed == nil {
		return fmt.Errorf("github: agent %q is not an option of the board's %q field — run `herdr-docket auth github` to add it", agent, s.cfg.agentField())
	}
	return s.setField(&t.project, t.item.ID, field, routed)
}

// SetPhase writes the board's own column, so a person watching the board can
// see what a run has in hand. It is display only: the fleet decides what to
// run from the issue's state, never from a column a human may have moved.
func (s *Source) SetPhase(id string, p work.Phase) error {
	t, err := s.locate(id)
	if err != nil {
		return err
	}
	field := t.project.field(s.cfg.statusField())
	word := s.phaseWord(p)
	if field == nil {
		return fmt.Errorf("github: the board has no %q field", s.cfg.statusField())
	}
	option := field.option(word)
	if option == nil {
		return fmt.Errorf("github: the board's %s field has no %q option — add it, or name yours in source.github.in_progress", s.cfg.statusField(), word)
	}
	return s.setField(&t.project, t.item.ID, field, option)
}

// unexported -----------------------------------------------------------------

// itemPage is how many of an issue's project items are read while looking for
// the one that is our board. projectItems cannot be filtered by project, so a
// page is read and matched; an issue belongs to a handful of boards, and one
// that belongs to more than ten is not a queue anyone can find work in.
const itemPage = 10

// writeTarget is a board with its fields, and the repository Create writes
// into. Create is the only caller that needs both, and it asks for them in one
// document so that a routing key is refused before an issue exists.
type writeTarget struct {
	project project
	repoID  string
}

// location is one existing task: the board, its node id, and its item there.
type location struct {
	project project
	issueID string
	item    itemNode
}

// project picks the board out of the two aliased answers. Exactly one of them
// is not null, and that is how the adapter learns whether the owner is an org
// without ever being told.
func (s *Source) project(org, user *boardHalf) (*project, error) {
	for _, half := range []*boardHalf{org, user} {
		if half != nil && half.ProjectV2 != nil {
			return half.ProjectV2, nil
		}
	}
	return nil, fmt.Errorf("github: no project %d under %q — check source.github.owner and .project", s.cfg.Project, s.cfg.Owner)
}

// item finds this issue's place on this board. An issue in the repository that
// is not on the board is not this fleet's work: it is somebody's inbox, or
// work another board is responsible for, and adding it here would be the fleet
// claiming a list it was not given.
func (s *Source) item(board *project, nodes []itemNode, number int) (itemNode, error) {
	for _, n := range nodes {
		if n.Project.ID == board.ID {
			return n, nil
		}
	}
	return itemNode{}, fmt.Errorf("github: issue %d is not on %s — add it to the board, or it is not this fleet's work", number, s.boardName())
}

// locate resolves everything a write needs about one task that already
// exists: the board and its fields, the issue's node id, and the item that is
// this fleet's. An issue that is not on the board stops here.
func (s *Source) locate(id string) (location, error) {
	number, err := issueNumber(id)
	if err != nil {
		return location{}, err
	}
	var out struct {
		Organization *boardHalf `json:"organization"`
		User         *boardHalf `json:"user"`
		Repository   struct {
			Issue *struct {
				ID           string `json:"id"`
				ProjectItems struct {
					Nodes []itemNode `json:"nodes"`
				} `json:"projectItems"`
			} `json:"issue"`
		} `json:"repository"`
	}
	if err := s.api.do("Target", targetQuery, s.issueVars(number), &out); err != nil {
		return location{}, err
	}
	issue := out.Repository.Issue
	if issue == nil {
		return location{}, fmt.Errorf("github: no issue %d in %s", number, s.cfg.Repo)
	}
	board, err := s.project(out.Organization, out.User)
	if err != nil {
		return location{}, err
	}
	item, err := s.item(board, issue.ProjectItems.Nodes, number)
	if err != nil {
		return location{}, err
	}
	return location{project: *board, issueID: issue.ID, item: item}, nil
}

func (s *Source) createTarget() (writeTarget, error) {
	var out struct {
		Organization *boardHalf `json:"organization"`
		User         *boardHalf `json:"user"`
		Repository   struct {
			ID string `json:"id"`
		} `json:"repository"`
	}
	if err := s.api.do("Create", createQuery, map[string]any{
		"owner":     s.cfg.Owner,
		"project":   s.cfg.Project,
		"repoOwner": s.repo.owner,
		"repoName":  s.repo.name,
	}, &out); err != nil {
		return writeTarget{}, err
	}
	if out.Repository.ID == "" {
		return writeTarget{}, fmt.Errorf("github: %s is not a repository this token can write to", s.cfg.Repo)
	}
	board, err := s.project(out.Organization, out.User)
	if err != nil {
		return writeTarget{}, err
	}
	return writeTarget{project: *board, repoID: out.Repository.ID}, nil
}

func (s *Source) setField(board *project, itemID string, field *field, option *option) error {
	var out struct {
		UpdateProjectV2ItemFieldValue struct {
			ProjectV2Item struct {
				ID string `json:"id"`
			} `json:"projectV2Item"`
		} `json:"updateProjectV2ItemFieldValue"`
	}
	return s.api.do("SetField", setFieldMutation, map[string]any{
		"project": board.ID,
		"item":    itemID,
		"field":   field.ID,
		"option":  option.ID,
	}, &out)
}

// phase turns the board's column into the fleet's word for it. The words the
// fleet knows are translated and anything else is passed through as written,
// because a column a human invented is still worth showing.
//
// Open always decides. The column only ever describes: a task in a column
// called Done that nobody closed is still work the fleet will run.
func (s *Source) phase(status string, open bool) string {
	if !open {
		// Without the thread, the poll cannot say why a task closed; the
		// column is the board's guess at it, and is believed only when it
		// names an ending. Get reads what the run actually said.
		if work.Verdict(strings.ToLower(status)).Known() {
			return status
		}
		return ""
	}
	switch status {
	case "", "Todo", "To Do":
		return string(work.PhaseTodo)
	case s.cfg.inProgress():
		return string(work.PhaseInProgress)
	}
	return status
}

// phaseWord is the column's word for a phase the fleet wants to show. Only the
// phases the fleet writes ever reach it, so the To Do and Done spellings are
// GitHub's defaults and nothing more.
func (s *Source) phaseWord(p work.Phase) string {
	switch p {
	case work.PhaseTodo:
		return "Todo"
	case work.PhaseInProgress:
		return s.cfg.inProgress()
	default:
		return string(p)
	}
}

// notes renders the thread. The author is part of the note: a conversation the
// fleet can read but not attribute is not a conversation.
func notes(nodes []comment) string {
	parts := make([]string, 0, len(nodes))
	for _, c := range nodes {
		who := c.Author.Login
		if who == "" {
			who = "unknown"
		}
		parts = append(parts, who+": "+strings.TrimSpace(c.Body))
	}
	return strings.Join(parts, "\n\n")
}

// verdictOf reads the newest verdict comment, if there is one. Newest first,
// so a task that was closed, reopened and closed again reports the latest
// thing said about it.
func verdictOf(thread string) work.Verdict {
	blocks := strings.Split(thread, "\n\n")
	for i := len(blocks) - 1; i >= 0; i-- {
		line := strings.TrimSpace(blocks[i])
		_, rest, ok := strings.Cut(line, verdictPrefix)
		if !ok {
			continue
		}
		if v := work.Verdict(strings.ToLower(strings.TrimSpace(rest))); v.Known() {
			return v
		}
	}
	return ""
}

// checklist is a markdown task list item: the one place an issue says what
// "done" means. An indented box is still a box.
var checklist = regexp.MustCompile(`^\s*[-*+]\s+\[([ xX])\]\s*(.+?)\s*$`)

// criteria reads the body's checklist as acceptance criteria, in the order it
// was written, numbered from one.
func criteria(body string) []work.Criterion {
	var out []work.Criterion
	for _, line := range strings.Split(body, "\n") {
		m := checklist.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		out = append(out, work.Criterion{
			Index:   len(out) + 1,
			Text:    m[2],
			Checked: m[1] != " ",
		})
	}
	return out
}

// issueNumber reads a task id. Ids are issue numbers and nothing else: they
// are what a person types after a #, and what the issue's own timeline calls
// it. A node id would be a second, unreadable name for the same thing.
func issueNumber(id string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(id))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("github: %q is not an issue number", id)
	}
	return n, nil
}

func (s *Source) issueVars(number int) map[string]any {
	return map[string]any{
		"owner":       s.cfg.Owner,
		"project":     s.cfg.Project,
		"repoOwner":   s.repo.owner,
		"repoName":    s.repo.name,
		"issue":       number,
		"agentField":  s.cfg.agentField(),
		"statusField": s.cfg.statusField(),
	}
}

// boardName names the board the way a person would, for error messages.
func (s *Source) boardName() string {
	return fmt.Sprintf("project %d of %s", s.cfg.Project, s.cfg.Owner)
}
