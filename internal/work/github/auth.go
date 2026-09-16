package github

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
)

// tokenEnv is the first place a token is looked for. It is the answer for CI,
// and for any machine where the fleet should not depend on a developer's gh
// login.
const tokenEnv = "HERDR_DOCKET_GITHUB_TOKEN"

// tokenSource hands out the access token, resolving it once and remembering it.
//
// The remembering is not an optimisation. The daemon and the board build a
// fresh Source on every tick — deliberately, so a config change cannot split
// the queue that was read from the queue written back — so resolving per
// construction would run `gh auth token` sixteen times a minute, forever. A
// token cannot change within one process except by being revoked, and that is
// what invalidate is for: a 401 empties the memo and the next call resolves
// again.
//
// A resolution *failure* is never remembered. gh being briefly unhappy is not
// a fact that should outlive the tick that saw it.
type tokenSource struct {
	store fileStore
	// gh asks the gh CLI for its token, and env reads the environment. Both
	// are fields so the resolution order is testable without a gh on PATH.
	gh  func() (string, error)
	env func(string) string

	mu       sync.Mutex
	token    string
	resolved bool
}

func defaultTokens() *tokenSource {
	return &tokenSource{store: defaultStore(), gh: ghToken, env: os.Getenv}
}

// access returns the token, resolving it on the first call after a
// construction or an invalidate.
func (t *tokenSource) access() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.resolved {
		return t.token, nil
	}
	tok, err := t.resolve()
	if err != nil {
		return "", err
	}
	t.token, t.resolved = tok, true
	return tok, nil
}

// invalidate forgets the resolved token, so that the next call resolves a
// fresh one. This is how a revoked or rotated token heals: the request that
// failed with 401 is tried once more against whatever is true now.
func (t *tokenSource) invalidate() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.token, t.resolved = "", false
}

// resolve reads the token from the environment, then from gh, then from the
// file `auth github` writes. The order is deliberate: the environment is the
// deployment's word and beats a developer's login, and gh beats the stored
// copy because gh is the thing that rotates.
func (t *tokenSource) resolve() (string, error) {
	if tok := strings.TrimSpace(t.env(tokenEnv)); tok != "" {
		return tok, nil
	}
	var ghErr error
	if tok, err := t.gh(); err != nil {
		ghErr = err
	} else if tok = strings.TrimSpace(tok); tok != "" {
		return tok, nil
	}
	stored, err := t.store.load()
	if err != nil {
		return "", err
	}
	if stored = strings.TrimSpace(stored); stored != "" {
		return stored, nil
	}
	msg := fmt.Sprintf("github: no token — set %s, run `gh auth login --scopes project`, or store one with `herdr-docket auth github --token <pat>`", tokenEnv)
	if ghErr != nil {
		msg += fmt.Sprintf(" (gh said: %v)", ghErr)
	}
	return "", errors.New(msg)
}

// ghToken asks the gh CLI for the token it holds for github.com. gh is the
// documented way to get one with the scopes a board needs, so when it is there
// and logged in, the fleet asks for nothing else.
func ghToken() (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", errors.New("gh is not on PATH")
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		// gh explains itself on stderr — "please run gh auth login" — and
		// that sentence is the whole reason the fleet asked it.
		var failed *exec.ExitError
		if errors.As(err, &failed) && len(failed.Stderr) > 0 {
			return "", fmt.Errorf("gh auth token: %s", strings.TrimSpace(string(failed.Stderr)))
		}
		return "", fmt.Errorf("gh auth token: %w", err)
	}
	return string(out), nil
}

// Login signs the fleet in to a board: it checks the token, provisions the
// field that carries the routing key, and stores a token that was handed to
// it.
//
// The provisioning happens here rather than in Create for a reason. A
// single-select field's options can only be written as a whole set, so adding
// one means reading every option, appending, and writing them all back — and a
// fleet that did that lazily would be rewriting the board's schema in the
// middle of a run, racing a human who is editing the same board. Once, at
// sign-in, with the whole set in hand, is the only place it is honest.
func Login(c Config, agents []string, handed string, out io.Writer) error {
	s, err := New(c)
	if err != nil {
		return err
	}
	return s.login(agents, handed, out)
}

// login is the sign-in itself, on a Source that already knows how to talk to a
// board. Tests drive this one directly: everything above it is configuration.
func (s *Source) login(agents []string, handed string, out io.Writer) error {
	if handed = strings.TrimSpace(handed); handed != "" {
		// A token given on the command line is the one answer that does not
		// survive the process, so it is the one worth writing down — but only
		// once GitHub has answered with it. A typo must not leave a bad token
		// in credentials.yaml, where every later run would find it and report
		// GitHub's complaint instead of telling the person to sign in.
		s.api.tokens = fixedToken(handed)
	}

	var viewer struct {
		Viewer struct {
			Login string `json:"login"`
		} `json:"viewer"`
	}
	if err := s.api.do("Viewer", viewerQuery, nil, &viewer); err != nil {
		return err
	}
	if viewer.Viewer.Login == "" {
		return errors.New("github: the token answered no login — is it still valid?")
	}
	fmt.Fprintf(out, "Signed in to GitHub as %s\n", viewer.Viewer.Login)
	if handed != "" {
		// Nothing else is stored: the environment and gh are already durable,
		// and copying them here would only make a second token to expire.
		store := defaultStore()
		if err := store.save(handed); err != nil {
			return err
		}
		fmt.Fprintf(out, "Token stored in %s\n", store.file.Path)
	}

	board, err := s.board()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Board: %s (%s)\n", board.Title, s.boardName())

	if err := s.provision(board, agents, out); err != nil {
		return err
	}
	s.noteStatus(board, out)
	return nil
}

// fixedToken is a token source with nothing left to resolve: the caller handed
// one over, and looking anywhere else would be looking in the wrong place.
func fixedToken(tok string) *tokenSource {
	return &tokenSource{
		env: func(string) string { return tok },
		gh:  func() (string, error) { return "", nil },
	}
}

// board is the configured project with its fields: what every write needs, and
// what sign-in provisions.
func (s *Source) board() (*project, error) {
	var out struct {
		Organization *boardHalf `json:"organization"`
		User         *boardHalf `json:"user"`
	}
	if err := s.api.do("Project", projectQuery, map[string]any{
		"owner":   s.cfg.Owner,
		"project": s.cfg.Project,
	}, &out); err != nil {
		return nil, err
	}
	return s.project(out.Organization, out.User)
}

// provision makes the board able to carry the routing key: an Agent
// single-select field with one option per agent.
//
// Nothing is ever removed. An option that disappears from the set takes every
// item's value with it, silently, for every column that used it — and an agent
// that is no longer described is a question for the person who stopped
// describing it, not for a sign-in to settle.
func (s *Source) provision(board *project, agents []string, out io.Writer) error {
	if len(agents) == 0 {
		return errors.New("github: no agents to route to — describe them in agents/<name>/AGENT.md first")
	}
	agents = append([]string(nil), agents...)
	sort.Strings(agents)

	name := s.cfg.agentField()
	f := board.field(name)
	if f == nil {
		if _, err := s.createField(board, name, agents); err != nil {
			return err
		}
		fmt.Fprintf(out, "Created the %s field, carrying %s\n", name, strings.Join(agents, ", "))
		return nil
	}
	if !f.singleSelect() {
		return fmt.Errorf("github: the board's %q field is a %s, not a single select — routing needs one option per agent", name, f.Typename)
	}
	missing := missingOptions(f, agents)
	if len(missing) == 0 {
		fmt.Fprintf(out, "The %s field already carries %s\n", name, strings.Join(agents, ", "))
		return nil
	}
	if err := s.addOptions(f, missing); err != nil {
		return err
	}
	fmt.Fprintf(out, "The %s field now carries %s (added %s)\n", name, strings.Join(agents, ", "), strings.Join(missing, ", "))
	return nil
}

// missingOptions is the agents the board cannot route to yet.
func missingOptions(f *field, agents []string) []string {
	var missing []string
	for _, agent := range agents {
		if f.option(agent) == nil {
			missing = append(missing, agent)
		}
	}
	return missing
}

// optionColors is what a new option is painted with, cycled. GitHub picks a
// colour when a person makes one in the browser, and a field written through
// the API has to name one.
var optionColors = []string{"BLUE", "GREEN", "YELLOW", "ORANGE", "PINK", "PURPLE", "RED", "GRAY"}

// colored turns names into options, colouring each in turn. offset is where to
// pick the cycle up: options already on the field keep the colours they have,
// so a second sign-in colours only what it adds.
func colored(names []string, offset int) []map[string]any {
	options := make([]map[string]any, 0, len(names))
	for i, name := range names {
		options = append(options, map[string]any{
			"name":  name,
			"color": optionColors[(offset+i)%len(optionColors)],
		})
	}
	return options
}

// existingOptions is every option already on the field, spelled the way the
// update mutation wants them. Their ids are carried over: an option sent
// without its id is a new option, and GitHub clears the item values that
// pointed at the old one.
func existingOptions(f *field) []map[string]any {
	options := make([]map[string]any, 0, len(f.Options))
	for _, o := range f.Options {
		colour := o.Color
		if colour == "" {
			colour = optionColors[len(options)%len(optionColors)]
		}
		options = append(options, map[string]any{
			"id":          o.ID,
			"name":        o.Name,
			"color":       colour,
			"description": o.Description,
		})
	}
	return options
}

func (s *Source) createField(board *project, name string, agents []string) (*field, error) {
	var out struct {
		CreateProjectV2Field struct {
			ProjectV2Field *field `json:"projectV2Field"`
		} `json:"createProjectV2Field"`
	}
	err := s.api.do("CreateField", createFieldMutation, map[string]any{
		"project": board.ID,
		"name":    name,
		"options": colored(agents, 0),
	}, &out)
	return out.CreateProjectV2Field.ProjectV2Field, err
}

func (s *Source) addOptions(f *field, names []string) error {
	options := append(existingOptions(f), colored(names, len(f.Options))...)
	var out struct {
		UpdateProjectV2Field struct {
			ProjectV2Field *field `json:"projectV2Field"`
		} `json:"updateProjectV2Field"`
	}
	return s.api.do("UpdateField", updateFieldMutation, map[string]any{
		"field":   f.ID,
		"options": options,
	}, &out)
}

// noteStatus says what the board will and will not show. The Status column is
// display only — nothing the fleet decides reads it — so a board without the
// word for work in hand is quieter, not broken.
func (s *Source) noteStatus(board *project, out io.Writer) {
	f := board.field(s.cfg.statusField())
	switch {
	case f == nil:
		fmt.Fprintf(out, "Note: no %q field on the board, so runs will not show as in hand. Set source.github.status_field if it is named something else.\n", s.cfg.statusField())
	case !f.singleSelect():
		fmt.Fprintf(out, "Note: the board's %q field is a %s, not a single select, so runs will not show as in hand.\n", s.cfg.statusField(), f.Typename)
	case f.option(s.cfg.inProgress()) == nil:
		fmt.Fprintf(out, "Note: no %q option on %s, so runs will not show as in hand. Add it, or set source.github.in_progress to the column's word.\n", s.cfg.inProgress(), s.cfg.statusField())
	}
}
