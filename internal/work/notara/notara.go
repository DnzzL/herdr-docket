// Package notara adapts a Notara database to work.Source: a card is a
// record, its Status and Agent selects carry the lifecycle and the routing
// key, its page's to-dos are the acceptance criteria and its comment thread
// is the conversation. Notara knows nothing of runs (its ADR-012): what a
// status word means is this fleet's `statuses:` block, read through
// work.Vocabulary like a Backlog.md project's (ADR-0022). Its kind in
// fleet.yaml is `notara`.
package notara

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/DnzzL/herdr-docket/internal/work"
)

// Field names on the database, as Notara's Backlog.md import creates them. A
// database has to be told the routing and status fields only when a human
// named them differently; labels and priority are read when present.
const (
	defaultAgentField  = "Agent"
	defaultStatusField = "Status"
	labelsField        = "Labels"
	priorityField      = "Priority"
)

// Config is the `source.notara` block of fleet.yaml: which Notara, and which
// database in it is the queue.
type Config struct {
	URL         string `yaml:"url"`          // the server, e.g. http://localhost:3100
	Workspace   string `yaml:"workspace"`    // workspace id
	Database    string `yaml:"database"`     // database id
	AgentField  string `yaml:"agent_field"`  // default Agent
	StatusField string `yaml:"status_field"` // default Status
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

// Source is a work.Source backed by one Notara database.
type Source struct {
	cfg   Config
	vocab work.Vocabulary
	base  string // worktree_base:, the only branch a hosted queue can name
	api   *api
}

// New builds a Source. It checks the configuration and talks to nothing: the
// fleet builds a Source on every tick, and a tick must not fail because the
// server is briefly down. vocab is required — a Notara database has no
// lifecycle of its own to default to.
func New(c Config, vocab work.Vocabulary, base string) (*Source, error) {
	c.URL = strings.TrimRight(strings.TrimSpace(c.URL), "/")
	var errs []error
	if c.URL == "" {
		errs = append(errs, errors.New("notara: source.notara.url is required — the server, e.g. http://localhost:3100"))
	}
	if strings.TrimSpace(c.Workspace) == "" {
		errs = append(errs, errors.New("notara: source.notara.workspace is required — the workspace id"))
	}
	if strings.TrimSpace(c.Database) == "" {
		errs = append(errs, errors.New("notara: source.notara.database is required — the id of the database that is the queue"))
	}
	if vocab == (work.Vocabulary{}) {
		errs = append(errs, errors.New("notara: statuses: is required — a Notara database has no lifecycle the fleet can assume"))
	} else if err := vocab.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return &Source{
		cfg:   c,
		vocab: vocab,
		base:  strings.TrimSpace(base),
		api: &api{
			base:  c.URL + "/api/v1/workspaces/" + c.Workspace + "/databases/" + c.Database,
			http:  &http.Client{Timeout: requestTimeout},
			token: defaultTokens().access,
		},
	}, nil
}

// List reads every card of the database in one request: Notara's list has no
// pages, and a backlog of a few hundred cards is one answer. No bodies, no
// comments — those are Get's.
func (s *Source) List() ([]work.Task, error) {
	var recs []rec
	if err := s.api.do(http.MethodGet, "/records", nil, &recs); err != nil {
		return nil, err
	}
	out := make([]work.Task, 0, len(recs))
	for _, r := range recs {
		if r.IsDeleted {
			continue
		}
		out = append(out, s.asTask(r))
	}
	return out, nil
}

// Get reads one card in full: its document, then its thread.
func (s *Source) Get(id string) (work.Task, error) {
	var d doc
	if err := s.api.do(http.MethodGet, record(id), nil, &d); err != nil {
		return work.Task{}, err
	}
	var cs []comment
	if err := s.api.do(http.MethodGet, record(id)+"/comments", nil, &cs); err != nil {
		return work.Task{}, err
	}
	it := s.asTask(d.rec)
	it.Body = d.Body
	for i, c := range d.Criteria {
		it.Criteria = append(it.Criteria, work.Criterion{Index: i + 1, Text: c.Text, Checked: c.Checked})
	}
	it.Notes = notes(cs)
	return it, nil
}

// Create files a card straight into the pickup word. Notara's select has no
// default status, and a card with none is open to no one: the fleet's whitelist
// never reads it, and nothing would ever claim it (ADR-0022).
func (s *Source) Create(title, body, assignee string) (string, error) {
	fields := map[string]any{s.cfg.statusField(): s.vocab.Todo}
	if assignee != "" {
		fields[s.cfg.agentField()] = assignee
	}
	var d doc
	if err := s.api.do(http.MethodPost, "/records", map[string]any{"title": title, "body": body, "fields": fields}, &d); err != nil {
		return "", err
	}
	return d.ShortID, nil
}

// CreateTodo is Create: the one write already lands in the pickup status,
// and saying so is what lets the CLI report it honestly (ADR-0012).
func (s *Source) CreateTodo(title, body, assignee string) (string, error) {
	return s.Create(title, body, assignee)
}

// Comment appends to the card's thread. Notara signs it with the key's user
// and the key's name.
func (s *Source) Comment(id, text string) error {
	return s.api.do(http.MethodPost, record(id)+"/comments", map[string]string{"body": text}, nil)
}

// Close writes the verdict's status word. An unknown verdict is refused
// before anything is written: the fleet reads Open, and a typo read as a
// close would throw the work away.
func (s *Source) Close(id string, v work.Verdict) error {
	if !v.Known() {
		return fmt.Errorf("close %s: unknown verdict %q", id, v)
	}
	return s.setField(id, s.cfg.statusField(), s.vocab.Status(v))
}

// SetPhase shows the card as being worked on — the one phase the fleet
// writes, display only. A database with no word for it is left alone.
func (s *Source) SetPhase(id string, phase work.Phase) error {
	if phase != work.PhaseInProgress {
		return fmt.Errorf("phase %q: the notara adapter does not know it", phase)
	}
	if s.vocab.InProgress == "" {
		return nil
	}
	return s.setField(id, s.cfg.statusField(), s.vocab.InProgress)
}

// Assign re-routes the card. An empty agent clears the field — Notara's empty
// select value is "", and null is refused — which hands the card to the
// queue's default agent.
func (s *Source) Assign(id, agent string) error {
	return s.setField(id, s.cfg.agentField(), agent)
}

// WriteCriteria appends to-dos after the card's existing criteria.
func (s *Source) WriteCriteria(id string, nc []string) error {
	if len(nc) == 0 {
		return nil
	}
	return s.api.do(http.MethodPost, record(id)+"/criteria", map[string]any{"criteria": nc}, nil)
}

// BaseBranch is the source's worktree_base:, the only answer a hosted queue
// has: there is no repo behind a database to derive one from.
func (s *Source) BaseBranch(string) (string, error) {
	if s.base == "" {
		return "", errors.New("notara: a hosted queue has no repo to name a branch from — set worktree_base: on the source")
	}
	return s.base, nil
}

func (s *Source) setField(id, field, v string) error {
	return s.api.do(http.MethodPatch, record(id), map[string]any{"fields": map[string]string{field: v}}, nil)
}

// asTask maps a card onto the fleet's vocabulary. Every reading of a status
// word goes through the vocabulary, so a triage column stays the humans'.
func (s *Source) asTask(r rec) work.Task {
	status := text(r.Fields[s.cfg.statusField()])
	return work.Task{
		ID:        r.ShortID,
		Title:     r.Title,
		Assignee:  text(r.Fields[s.cfg.agentField()]),
		Open:      s.vocab.Open(status),
		Blocked:   s.vocab.Parked(status),
		Phase:     s.vocab.Phase(status),
		Verdict:   s.vocab.VerdictOf(status),
		Priority:  rank(text(r.Fields[priorityField])),
		CreatedAt: r.CreatedAt,
		Labels:    texts(r.Fields[labelsField]),
	}
}

// rank reads the Priority select Notara's import creates. Higher is more
// urgent; anything else, including none, is the backend having no opinion.
func rank(priority string) int {
	switch priority {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	}
	return 0
}

// notes is the thread as the prompt reads it, a comment written through an
// API key prefixed with the key's name — which agent said it.
func notes(cs []comment) string {
	parts := make([]string, 0, len(cs))
	for _, c := range cs {
		body := strings.TrimSpace(c.Body)
		if body == "" {
			continue
		}
		if c.KeyName != "" {
			body = c.KeyName + ": " + body
		}
		parts = append(parts, body)
	}
	return strings.Join(parts, "\n\n")
}

// text is a select's value. A field with no value is absent from Notara's
// map; anything not a string is not a value the fleet routes on.
func text(v any) string {
	s, _ := v.(string)
	return s
}

func texts(v any) []string {
	xs, _ := v.([]any)
	var out []string
	for _, x := range xs {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// rec is a card as Notara lists it. Field names are the API's.
type rec struct {
	ID        string         `json:"id"`
	ShortID   string         `json:"shortId"`
	Title     string         `json:"title"`
	IsDeleted bool           `json:"isDeleted"`
	CreatedAt string         `json:"createdAt"`
	Fields    map[string]any `json:"fields"`
}

// doc is a card as Notara reads one back: the listing, plus its page.
type doc struct {
	rec
	Body     string `json:"body"`
	Criteria []struct {
		ID      string `json:"id"`
		Text    string `json:"text"`
		Checked bool   `json:"checked"`
	} `json:"criteria"`
}

type comment struct {
	Body    string `json:"body"`
	KeyName string `json:"keyName"`
}
