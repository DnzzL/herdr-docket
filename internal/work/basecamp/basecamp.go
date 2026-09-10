// Package basecamp adapts a Basecamp project to work.Source. Basecamp has no
// labels, so the fleet's routing key rides on the container instead: one
// to-do list per agent. Rich text arrives as HTML and leaves as markdown.
package basecamp

import (
	"fmt"
	stdhtml "html"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/work"
)

// Config is the `source.basecamp` block of fleet.yaml.
//
// Basecamp to-dos have no labels, so the routing key rides on the container:
// one to-do list per agent. Which list belongs to which agent is the entire
// configuration, because it is the entire routing table.
type Config struct {
	// AccountID is the numeric Basecamp account — the first element of every
	// API path. YAML quoting is optional; it is read as text either way.
	AccountID string `yaml:"account_id"`
	// Lists maps an agent to the id of the to-do list that carries its work.
	Lists map[string]string `yaml:"lists"`
}

// Source is a work.Source backed by a Basecamp project.
type Source struct {
	api    *api
	agents []string          // sorted, so List is stable between runs
	listID map[string]string // agent  -> to-do list id
	agent  map[string]string // list id -> agent
}

// New builds a Source on a Basecamp account. This is the only place the
// adapter's configuration is checked, so a fleet that is going to be
// misrouted finds out at startup rather than at the first poll.
func New(c Config) (*Source, error) {
	if strings.TrimSpace(c.AccountID) == "" {
		return nil, fmt.Errorf("basecamp: account_id is required — it is the account the fleet's to-do lists live in")
	}
	if len(c.Lists) == 0 {
		return nil, fmt.Errorf("basecamp: no lists configured — Basecamp has no labels, so every agent needs a to-do list of its own")
	}
	s := &Source{
		api: &api{
			base:   APIHost + "/" + strings.TrimSpace(c.AccountID),
			http:   http.DefaultClient,
			tokens: &credentials{store: defaultStore(), oauth: defaultOAuth(), now: time.Now},
			sleep:  time.Sleep,
			now:    time.Now,
		},
		listID: make(map[string]string, len(c.Lists)),
		agent:  make(map[string]string, len(c.Lists)),
	}
	for agent, list := range c.Lists {
		agent, list = strings.TrimSpace(agent), strings.TrimSpace(list)
		if agent == "" || list == "" {
			return nil, fmt.Errorf("basecamp: a list of %q has no list id", agent)
		}
		s.listID[agent] = list
		s.agent[list] = agent
		s.agents = append(s.agents, agent)
	}
	sort.Strings(s.agents)
	return s, nil
}

// List reads every configured to-do list. One request per agent: Basecamp has
// no query that spans lists, so the routing table is also the read plan.
func (s *Source) List() ([]work.Item, error) {
	var items []work.Item
	for _, agent := range s.agents {
		var todos []todo
		if err := s.api.get("/todolists/"+s.listID[agent]+"/todos.json", &todos); err != nil {
			return nil, fmt.Errorf("basecamp: listing what is assigned to %s: %w", agent, err)
		}
		for _, t := range todos {
			it := item(t)
			it.Assignee = agent
			items = append(items, it)
		}
	}
	return items, nil
}

// Get returns one to-do in full. The comments are a second request because
// Basecamp keeps them as recordings of their own, not as a field of the to-do.
func (s *Source) Get(id string) (work.Item, error) {
	var t todo
	if err := s.api.get("/todos/"+id+".json", &t); err != nil {
		return work.Item{}, err
	}
	var comments []comment
	if err := s.api.get("/recordings/"+id+"/comments.json", &comments); err != nil {
		return work.Item{}, err
	}
	it := item(t)
	it.Assignee = s.agent[strconv.Itoa(t.Parent.ID)]
	it.Body = body(t)
	it.Notes = notes(comments)
	it.Criteria = criteria(t.Steps)
	return it, nil
}

// Create posts a to-do into the list that carries the assignee. That is the
// only way to tell Basecamp who the work is for, and the reason an unknown
// agent is an error rather than a default: a to-do in the wrong list is work
// handed to the wrong agent.
func (s *Source) Create(title, body, assignee string) (string, error) {
	list, ok := s.listID[assignee]
	if !ok {
		return "", fmt.Errorf("basecamp: no list carries %q — add it to source.basecamp.lists", assignee)
	}
	var t todo
	if err := s.api.post("/todolists/"+list+"/todos.json", content{Title: title, Content: richText(body)}, &t); err != nil {
		return "", err
	}
	return strconv.Itoa(t.ID), nil
}

// Comment appends to the to-do's stream of comments.
func (s *Source) Comment(id, text string) error {
	return s.api.post("/recordings/"+id+"/comments.json", content{Content: richText(text)}, nil)
}

// Close completes the to-do, then comments the verdict. Basecamp has one word
// for an ending — completed — so a failed or blocked item would otherwise
// look exactly like a finished one; the comment is the only place left to say
// which it was. The completion comes first so that a failure to comment
// leaves an item the fleet has closed, not one it will run again.
func (s *Source) Close(id string, v work.Verdict) error {
	if err := s.api.post("/todos/"+id+"/completion.json", nil, nil); err != nil {
		return err
	}
	return s.Comment(id, "Verdict: "+string(v))
}

// item maps a Basecamp to-do onto the fleet's vocabulary. Assignee is left to
// the caller: only the caller knows which list the to-do came out of, and on
// Basecamp the list is the assignee.
func item(t todo) work.Item {
	return work.Item{
		ID:        strconv.Itoa(t.ID),
		Title:     t.Title,
		Open:      !closed(t),
		Phase:     phase(t),
		Ordinal:   float64(t.Position),
		CreatedAt: t.CreatedAt,
	}
}

// closed reads Basecamp's own idea of finished. The completed flag is the
// direct answer; status covers to-dos that left their list another way
// (archived, trashed) without ever being ticked off.
func closed(t todo) bool {
	switch t.Status {
	case "completed", "archived", "trashed":
		return true
	}
	return t.Completed
}

// The fleet's display words for the two states Basecamp can express. A
// to-do is open or it is closed, and these are the words the board already
// uses for those two conditions.
const (
	phaseOpen   = "To Do"
	phaseClosed = "Done"
)

// phase renders Basecamp's status in the fleet's vocabulary. Calling an open
// to-do "To Do" is not a claim that nobody has started it — Basecamp simply
// has no word for started, which is why this adapter writes no phase at all.
func phase(t todo) string {
	if closed(t) {
		return phaseClosed
	}
	return phaseOpen
}

// body is the to-do's description in the fleet's words. Basecamp sends the
// rich text in content and a plain-text description that list responses
// truncate, so content wins whenever there is any.
func body(t todo) string {
	if md := htmlToMarkdown(t.Content); md != "" {
		return md
	}
	return strings.TrimSpace(t.Description)
}

// criteria reads Basecamp's steps as the fleet's acceptance criteria. A
// step's checkbox is the backend's own record of it, and criteria are
// read-only to the fleet: the verdict and its evidence go in the comment that
// closes the item, never back onto these.
func criteria(steps []step) []work.Criterion {
	sorted := append([]step(nil), steps...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Position < sorted[j].Position })
	out := make([]work.Criterion, 0, len(sorted))
	for i, st := range sorted {
		out = append(out, work.Criterion{Index: i + 1, Text: st.Title, Checked: st.Completed})
	}
	return out
}

// notes is the comment stream as the prompt will read it. Basecamp has
// comments where Backlog.md has a notes field; both arrive here as the same
// thing.
func notes(comments []comment) string {
	var parts []string
	for _, c := range comments {
		text := htmlToMarkdown(c.Content)
		if text == "" {
			continue
		}
		if c.Creator.Name != "" {
			text = c.Creator.Name + ": " + text
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n")
}

// richText turns the fleet's plain text into the HTML Basecamp stores. The
// fleet writes prose, not markup: a body arrives as escaped paragraphs,
// which is exactly what was typed.
func richText(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	var b strings.Builder
	for _, para := range strings.Split(strings.TrimSpace(s), "\n\n") {
		b.WriteString("<div>" + strings.ReplaceAll(stdhtml.EscapeString(para), "\n", "<br>") + "</div>")
	}
	return b.String()
}

// content is the one shape Basecamp writes: a rich-text body, optionally
// with a title. A new to-do and a comment are both one of these.
type content struct {
	Title   string `json:"title,omitempty"`
	Content string `json:"content"`
}

// todo is the slice of Basecamp's to-do JSON the fleet reads. Field names and
// nesting are the API's; everything above this file sees work.Item instead.
type todo struct {
	ID          int     `json:"id"`
	Status      string  `json:"status"`
	Completed   bool    `json:"completed"`
	Title       string  `json:"title"`
	Content     string  `json:"content"`
	Description string  `json:"description"`
	Position    int     `json:"position"`
	CreatedAt   string  `json:"created_at"`
	Steps       []step  `json:"steps"`
	Parent      ref     `json:"parent"`
	Bucket      ref     `json:"bucket"`
	Assignees   []actor `json:"assignees"`
}

type step struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
	Position  int    `json:"position"`
}

type comment struct {
	ID        int    `json:"id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	Creator   actor  `json:"creator"`
}

type actor struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ref struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}
