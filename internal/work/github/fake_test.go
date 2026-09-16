package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// The board every test is about, named once so a test reads like a fleet.yaml.
const (
	testOwner     = "acme"
	testRepo      = "acme/widgets"
	testProject   = 3
	testProjectID = "PVT_board"
	testViewer    = "octocat"
)

// testConfig is the configuration a test drives the adapter with.
func testConfig() Config {
	return Config{Owner: testOwner, Project: testProject, Repo: testRepo}
}

// newSource is the adapter over a fake board, as the shared contract suite
// wants it: fresh on every call, and signed in — the Agent field exists with
// the routing keys the suite uses, because a board without it cannot carry a
// routing key at all and the suite would be testing the wrong thing.
func newSource(t *testing.T) *Source {
	t.Helper()
	f := newFakeBoard()
	f.provisionAgent("dev", "reviewer", "pm")
	return f.source(t)
}

// newFakeBoard is an empty repository and the board over it, with the Status
// column a new GitHub board comes with.
func newFakeBoard() *fakeBoard {
	f := &fakeBoard{
		owner:        testOwner,
		repo:         testRepo,
		project:      testProject,
		projectTitle: "Fleet",
		issues:       map[int]*fakeIssue{},
		viewer:       testViewer,
		fail:         map[string]string{},
	}
	f.fields = []*fakeField{f.newField("Status", "Todo", "In Progress", "Done")}
	return f
}

// source is a real adapter talking to the fake over HTTP: the document goes out
// as JSON and the answer comes back as GitHub would send it. Nothing about the
// adapter is stubbed, because a contract proved against a stub would only prove
// the stub.
func (f *fakeBoard) source(t *testing.T) *Source {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	s, err := New(testConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	s.api = &api{
		url:    srv.URL,
		http:   srv.Client(),
		tokens: fixedToken("test-token"),
		sleep:  func(d time.Duration) { f.slept = append(f.slept, d) },
		now:    func() time.Time { return time.Unix(1_700_000_000, 0) },
	}
	return s
}

// fakeBoard is an in-memory GitHub: a repository of issues and a Projects v2
// board over them, with enough of the API for the adapter to be driven through
// it exactly as the fleet drives it for real.
//
// It is deliberately stricter than a stub would be. It refuses a single-select
// write whose option does not exist, the way the real API does, because the
// thing most worth proving about the routing writes is that they fail loudly
// rather than clearing a field. It does not interpret the queue's search
// filter: the fake has one repository, and a test that cares asserts on the
// filter the adapter sent.
type fakeBoard struct {
	mu sync.Mutex

	owner        string
	repo         string
	viewer       string
	asUser       bool // the owner answers as a user rather than an organization
	project      int
	projectTitle string

	fields []*fakeField
	items  []*fakeItem
	issues map[int]*fakeIssue
	seq    int
	next   int // the highest issue number handed out

	// what a test can force
	fail         map[string]string // operation -> the error to answer with
	unauthorized int               // requests to refuse as unauthenticated
	throttled    int               // requests to refuse as rate limited
	limited      bool              // answer rate limits as a 200 carrying errors

	asked  []asked
	slept  []time.Duration
	bearer []string
}

// asked is one request as it arrived, which is how a test asserts on the shape
// of what went out and not only on what came back.
type asked struct {
	operation string
	vars      map[string]any
	query     string
}

type fakeField struct {
	typename string
	id       string
	name     string
	options  []gqlOption
}

type fakeItem struct {
	id     string
	number int // zero when the item carries something that is not an issue
	draft  bool
	agent  string
	status string
}

type fakeIssue struct {
	number    int
	title     string
	body      string
	state     string
	reason    string
	createdAt string
	comments  []gqlComment
}

// The shapes the fake answers with. They carry the JSON tags the API uses, not
// the adapter's read structs: what this serves is what the adapter would get
// from the real thing, so a tag the adapter gets wrong is a test that fails.
type gqlProject struct {
	ID     string              `json:"id,omitempty"`
	Title  string              `json:"title,omitempty"`
	Fields *gqlNodes[gqlField] `json:"fields,omitempty"`
	Items  *gqlNodes[gqlItem]  `json:"items,omitempty"`
}

type gqlNodes[T any] struct {
	Nodes []T `json:"nodes"`
}

type gqlField struct {
	Typename string      `json:"__typename"`
	ID       string      `json:"id,omitempty"`
	Name     string      `json:"name,omitempty"`
	Options  []gqlOption `json:"options,omitempty"`
}

type gqlOption struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
}

type gqlItem struct {
	Content *gqlIssue `json:"content,omitempty"`
	Agent   *gqlValue `json:"agent,omitempty"`
	Status  *gqlValue `json:"status,omitempty"`
	ID      string    `json:"id,omitempty"`
	Project *gqlRef   `json:"project,omitempty"`
}

type gqlValue struct {
	Name string `json:"name"`
}

type gqlRef struct {
	ID string `json:"id"`
}

type gqlIssue struct {
	Number       int                   `json:"number,omitempty"`
	Title        string                `json:"title,omitempty"`
	Body         string                `json:"body,omitempty"`
	State        string                `json:"state,omitempty"`
	CreatedAt    string                `json:"createdAt,omitempty"`
	Comments     *gqlNodes[gqlComment] `json:"comments,omitempty"`
	ProjectItems *gqlNodes[gqlItem]    `json:"projectItems,omitempty"`
}

type gqlComment struct {
	Body   string    `json:"body"`
	Author gqlAuthor `json:"author"`
}

type gqlAuthor struct {
	Login string `json:"login"`
}

// newField adds a single-select field with the given options.
func (f *fakeBoard) newField(name string, options ...string) *fakeField {
	field := &fakeField{typename: "ProjectV2SingleSelectField", id: f.id("PVTSSF"), name: name}
	for _, o := range options {
		field.options = append(field.options, gqlOption{ID: f.id("PVTSSO"), Name: o, Color: "GRAY"})
	}
	return field
}

// provisionAgent is what `auth github` leaves behind: the routing field, with
// one option per agent. A test that drives routing calls it, so the board it
// drives has been signed in to.
func (f *fakeBoard) provisionAgent(names ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fields = append(f.fields, f.newField("Agent", names...))
}

// add writes an issue and puts it on the board, the way a person does.
func (f *fakeBoard) add(title, body string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := f.seed(title, body)
	f.board(n, "")
	return n
}

// addClosed writes an issue that has already ended, with the verdict comment a
// run would have left.
func (f *fakeBoard) addClosed(title, body, verdict string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := f.seed(title, body)
	f.board(n, "")
	iss := f.issues[n]
	iss.state = "CLOSED"
	iss.reason = "COMPLETED"
	iss.comments = append(iss.comments, gqlComment{Body: verdictPrefix + verdict, Author: gqlAuthor{Login: f.viewer}})
	return n
}

// issue is one issue's whole state, for a test to read or to arrange. It is
// the fake's own state and not the adapter's, so a test can set up a board a
// person would have left behind.
func (f *fakeBoard) issue(number int) *fakeIssue {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.issues[number]
}

// setStatus puts an issue in one of the board's columns.
func (f *fakeBoard) setStatus(number int, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if item := f.itemOf(number); item != nil {
		item.status = status
	}
}

// end closes an issue the way GitHub's own workflow does: the issue ends, and
// the board's column follows it into Done. No verdict is written, because the
// automation is not the fleet.
func (f *fakeBoard) end(number int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	iss := f.issues[number]
	iss.state = "CLOSED"
	iss.reason = "COMPLETED"
	if item := f.itemOf(number); item != nil {
		item.status = "Done"
	}
}

// reopen puts a closed issue back in the queue, as a person would: open again,
// with its old thread still there.
func (f *fakeBoard) reopen(number int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	iss := f.issues[number]
	iss.state = "OPEN"
	iss.reason = ""
}

// addUnboarded writes an issue that is in the repository and on no board.
func (f *fakeBoard) addUnboarded(title string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.seed(title, "")
}

// addDraft puts a card on the board that is not an issue at all. An item whose
// content is a draft comes back with no issue fields, exactly as a pull request
// does: the document only asks for the ones an Issue has.
func (f *fakeBoard) addDraft() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items = append(f.items, &fakeItem{id: f.id("PVTI"), draft: true})
}

func (f *fakeBoard) seed(title, body string) int {
	f.next++
	n := f.next
	f.issues[n] = &fakeIssue{
		number:    n,
		title:     title,
		body:      body,
		state:     "OPEN",
		createdAt: "2026-01-0" + strconv.Itoa(n) + "T09:00:00Z",
	}
	return n
}

func (f *fakeBoard) board(number int, status string) {
	f.items = append(f.items, &fakeItem{id: f.id("PVTI"), number: number, status: status})
}

func (f *fakeBoard) id(prefix string) string {
	f.seq++
	return prefix + "_" + strconv.Itoa(f.seq)
}

// field is the board's field of that name, the way the API would answer.
func (f *fakeBoard) field(name string) *gqlField {
	for _, field := range f.fields {
		if field.name == name {
			out := &gqlField{Typename: field.typename, ID: field.id, Name: field.name}
			if field.typename == "ProjectV2SingleSelectField" {
				out.Options = append([]gqlOption(nil), field.options...)
			}
			return out
		}
	}
	return nil
}

func (f *fakeBoard) fieldsNode() *gqlNodes[gqlField] {
	out := &gqlNodes[gqlField]{}
	for _, field := range f.fields {
		out.Nodes = append(out.Nodes, *f.field(field.name))
	}
	return out
}

// itemOf is this issue's item, or nil when the board does not carry it.
func (f *fakeBoard) itemOf(number int) *fakeItem {
	for _, item := range f.items {
		if item.number == number && !item.draft {
			return item
		}
	}
	return nil
}

// ServeHTTP answers one GraphQL request. Everything is dispatched on the
// document's operation name, which is what a GraphQL server's log would show.
func (f *fakeBoard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query         string         `json:"query"`
		Variables     map[string]any `json:"variables"`
		OperationName string         `json:"operationName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "the fake needs a GraphQL request: "+err.Error(), http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, asked{operation: req.OperationName, vars: req.Variables, query: req.Query})
	f.bearer = append(f.bearer, r.Header.Get("Authorization"))

	switch {
	case f.unauthorized > 0:
		f.unauthorized--
		f.reply(w, http.StatusUnauthorized, map[string]any{"message": "Bad credentials"})
		return
	case f.throttled > 0:
		f.throttled--
		w.Header().Set("Retry-After", "2")
		f.reply(w, http.StatusTooManyRequests, map[string]any{"message": "You have exceeded a secondary rate limit"})
		return
	case f.limited:
		f.limited = false
		f.reply(w, http.StatusOK, map[string]any{
			"data":   nil,
			"errors": []any{map[string]any{"type": "RATE_LIMITED", "message": "API rate limit exceeded"}},
		})
		return
	}

	if msg, forced := f.fail[req.OperationName]; forced {
		f.reply(w, http.StatusOK, map[string]any{
			"data":   nil,
			"errors": []any{map[string]any{"type": "UNPROCESSABLE", "message": msg}},
		})
		return
	}

	data, err := f.answer(req.OperationName, req.Variables)
	if err != nil {
		f.reply(w, http.StatusOK, map[string]any{
			"data":   nil,
			"errors": []any{map[string]any{"type": "UNPROCESSABLE", "message": err.Error()}},
		})
		return
	}
	if data == nil {
		http.Error(w, "the fake does not know the operation "+req.OperationName, http.StatusNotImplemented)
		return
	}
	f.reply(w, http.StatusOK, map[string]any{"data": data})
}

func (f *fakeBoard) reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// halves answers the doubled question about a board's owner: the same project
// asked for as an organization and as a user. Exactly one of the two carries
// it, as the API answers.
func (f *fakeBoard) halves(vars map[string]any, build func(*gqlProject)) map[string]any {
	if varString(vars, "owner") != f.owner || varInt(vars, "project") != f.project {
		return map[string]any{"organization": nil, "user": nil}
	}
	p := &gqlProject{ID: testProjectID, Title: f.projectTitle}
	build(p)
	half := map[string]any{"projectV2": p}
	if f.asUser {
		return map[string]any{"organization": nil, "user": half}
	}
	return map[string]any{"organization": half, "user": nil}
}

func (f *fakeBoard) answer(operation string, vars map[string]any) (any, error) {
	switch operation {
	case "Viewer":
		return map[string]any{"viewer": map[string]any{"login": f.viewer}}, nil

	case "Project":
		return f.halves(vars, func(p *gqlProject) {
			p.Fields = f.fieldsNode()
		}), nil

	case "Items":
		return f.halves(vars, func(p *gqlProject) {
			p.Items = &gqlNodes[gqlItem]{Nodes: f.page(vars)}
		}), nil

	case "Issue":
		return f.issueData(vars), nil

	case "Target":
		return f.targetData(vars), nil

	case "Create":
		return f.createData(vars), nil

	case "CreateIssue":
		return f.createIssue(vars)

	case "SetField":
		return f.setField(vars)

	case "AddComment":
		return f.addComment(vars)

	case "Close":
		return f.close(vars)

	case "CreateField":
		return f.createField(vars)

	case "UpdateField":
		return f.updateField(vars)
	}
	return nil, nil
}

// page is the queue's page: the board's items, in the board's order, carrying
// the two columns the queue reads.
func (f *fakeBoard) page(vars map[string]any) []gqlItem {
	agentField, statusField := varString(vars, "agentField"), varString(vars, "statusField")
	nodes := make([]gqlItem, 0, len(f.items))
	for _, item := range f.items {
		node := gqlItem{}
		if item.draft {
			node.Content = &gqlIssue{}
		} else {
			issue := f.issues[item.number]
			node.Content = &gqlIssue{Number: issue.number, Title: issue.title, State: issue.state, CreatedAt: issue.createdAt}
		}
		if item.agent != "" && f.field(agentField) != nil {
			node.Agent = &gqlValue{Name: item.agent}
		}
		if item.status != "" && f.field(statusField) != nil {
			node.Status = &gqlValue{Name: item.status}
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func (f *fakeBoard) itemsNode(number int) *gqlNodes[gqlItem] {
	out := &gqlNodes[gqlItem]{}
	for _, item := range f.items {
		if item.draft || item.number != number {
			continue
		}
		node := gqlItem{ID: item.id, Project: &gqlRef{ID: testProjectID}}
		if item.agent != "" {
			node.Agent = &gqlValue{Name: item.agent}
		}
		if item.status != "" {
			node.Status = &gqlValue{Name: item.status}
		}
		out.Nodes = append(out.Nodes, node)
	}
	return out
}

func (f *fakeBoard) issueData(vars map[string]any) map[string]any {
	out := f.halves(vars, func(p *gqlProject) { p.ID = testProjectID })
	out["repository"] = map[string]any{"issue": f.issueNode(varInt(vars, "issue"), vars)}
	return out
}

// issueNode is one issue in full. A repository or an issue that is not there
// answers null, which is what the adapter has to report honestly rather than
// as a zero task.
func (f *fakeBoard) issueNode(number int, vars map[string]any) any {
	issue := f.issues[number]
	if issue == nil || !f.inRepo(vars) {
		return nil
	}
	return &gqlIssue{
		Number:       issue.number,
		Title:        issue.title,
		Body:         issue.body,
		State:        issue.state,
		CreatedAt:    issue.createdAt,
		Comments:     &gqlNodes[gqlComment]{Nodes: issue.comments},
		ProjectItems: f.itemsNode(number),
	}
}

func (f *fakeBoard) targetData(vars map[string]any) map[string]any {
	out := f.halves(vars, func(p *gqlProject) { p.Fields = f.fieldsNode() })
	number := varInt(vars, "issue")
	issue := f.issues[number]
	if issue == nil || !f.inRepo(vars) {
		out["repository"] = map[string]any{"issue": nil}
		return out
	}
	out["repository"] = map[string]any{"issue": map[string]any{
		"id":           f.issueID(number),
		"projectItems": f.itemsNode(number),
	}}
	return out
}

func (f *fakeBoard) createData(vars map[string]any) map[string]any {
	out := f.halves(vars, func(p *gqlProject) { p.Fields = f.fieldsNode() })
	repo := ""
	if f.inRepo(vars) {
		repo = "R_" + f.repo
	}
	out["repository"] = map[string]any{"id": repo}
	return out
}

// createIssue writes the issue and puts it on the board in one mutation, as
// the adapter asks it to. projectV2Ids is what makes it one step; a fake that
// ignored it would let a two-step implementation pass.
func (f *fakeBoard) createIssue(vars map[string]any) (any, error) {
	if varString(vars, "repo") == "" {
		return nil, fmt.Errorf("createIssue: no repository")
	}
	if varString(vars, "project") != testProjectID {
		return nil, fmt.Errorf("createIssue: no such project")
	}
	// Written issues are numbered high, away from the ones a test seeded by
	// hand, so that a test can tell the adapter's issue from its own.
	f.next += 100
	n := f.next
	f.issues[n] = &fakeIssue{
		number:    n,
		title:     varString(vars, "title"),
		body:      varString(vars, "body"),
		state:     "OPEN",
		createdAt: "2026-02-01T09:00:00Z",
	}
	item := &fakeItem{id: f.id("PVTI"), number: n}
	f.items = append(f.items, item)
	issue := f.issues[n]
	return map[string]any{"createIssue": map[string]any{"issue": map[string]any{
		"id":           f.issueID(n),
		"number":       issue.number,
		"projectItems": f.itemsNode(n),
	}}}, nil
}

// setField is where the fake is strict on purpose: an option that is not on the
// field is refused, as the API refuses it. A silent no-op here would let a
// routing bug through and look like it worked.
func (f *fakeBoard) setField(vars map[string]any) (any, error) {
	field := f.fieldByID(varString(vars, "field"))
	if field == nil {
		return nil, fmt.Errorf("Could not resolve to a node with the global id of '%s'", varString(vars, "field"))
	}
	if field.typename != "ProjectV2SingleSelectField" {
		return nil, fmt.Errorf("Field %q does not take a single select value", field.name)
	}
	item := f.itemByID(varString(vars, "item"))
	if item == nil {
		return nil, fmt.Errorf("Could not resolve to a node with the global id of '%s'", varString(vars, "item"))
	}
	option := varString(vars, "option")
	known := false
	for _, o := range field.options {
		if o.ID == option {
			known = true
		}
	}
	if !known {
		return nil, fmt.Errorf("Could not resolve to a node with the global id of '%s'", option)
	}
	switch field.name {
	case "Agent":
		item.agent = optionName(field, option)
	default:
		item.status = optionName(field, option)
	}
	return map[string]any{"updateProjectV2ItemFieldValue": map[string]any{
		"projectV2Item": map[string]any{"id": item.id},
	}}, nil
}

func (f *fakeBoard) addComment(vars map[string]any) (any, error) {
	issue, err := f.subject(vars)
	if err != nil {
		return nil, err
	}
	f.seq++
	issue.comments = append(issue.comments, gqlComment{
		Body:   varString(vars, "body"),
		Author: gqlAuthor{Login: f.viewer},
	})
	return map[string]any{"addComment": map[string]any{
		"commentEdge": map[string]any{"node": map[string]any{"id": "IC_" + strconv.Itoa(f.seq)}},
	}}, nil
}

func (f *fakeBoard) close(vars map[string]any) (any, error) {
	issue, err := f.subject(vars)
	if err != nil {
		return nil, err
	}
	issue.state = "CLOSED"
	issue.reason = varString(vars, "reason")
	f.seq++
	comment := gqlComment{Body: varString(vars, "body"), Author: gqlAuthor{Login: f.viewer}}
	issue.comments = append(issue.comments, comment)
	return map[string]any{
		"closeIssue": map[string]any{"issue": map[string]any{
			"id": f.issueID(issue.number), "state": issue.state, "stateReason": issue.reason,
		}},
		"addComment": map[string]any{"commentEdge": map[string]any{"node": map[string]any{"id": "IC_" + strconv.Itoa(f.seq)}}},
	}, nil
}

func (f *fakeBoard) createField(vars map[string]any) (any, error) {
	name := varString(vars, "name")
	if f.field(name) != nil {
		return nil, fmt.Errorf("a field named %q already exists on this project", name)
	}
	field := &fakeField{typename: "ProjectV2SingleSelectField", id: f.id("PVTSSF"), name: name}
	field.options = f.options(vars)
	f.fields = append(f.fields, field)
	return map[string]any{"createProjectV2Field": map[string]any{"projectV2Field": *f.field(name)}}, nil
}

// updateField replaces the field's options with the ones it was sent, which is
// what the API does. An option sent without its id is a new option, and the
// value that pointed at the old one is gone — the reason the adapter sends the
// existing options back with their ids.
func (f *fakeBoard) updateField(vars map[string]any) (any, error) {
	field := f.fieldByID(varString(vars, "field"))
	if field == nil {
		return nil, fmt.Errorf("Could not resolve to a node with the global id of '%s'", varString(vars, "field"))
	}
	options := f.options(vars)
	f.fields[f.indexOf(field.name)].options = options
	f.replaceValues(field.name, options)
	return map[string]any{"updateProjectV2Field": map[string]any{"projectV2Field": *f.field(field.name)}}, nil
}

// replaceValues clears any item value whose option no longer exists. It is the
// API's most dangerous behaviour around fields, and the reason nothing here
// ever removes an option.
func (f *fakeBoard) replaceValues(name string, options []gqlOption) {
	names := map[string]bool{}
	for _, o := range options {
		names[o.Name] = true
	}
	for _, item := range f.items {
		if name == "Agent" && item.agent != "" && !names[item.agent] {
			item.agent = ""
		}
	}
}

// options is the option set as it was sent. An option that arrived without an
// id is new and gets one; one that arrived with its id keeps it, which is what
// keeps the items pointing at it alive.
func (f *fakeBoard) options(vars map[string]any) []gqlOption {
	sent, _ := vars["options"].([]any)
	out := make([]gqlOption, 0, len(sent))
	for _, raw := range sent {
		one, _ := raw.(map[string]any)
		o := gqlOption{
			ID:          asString(one["id"]),
			Name:        asString(one["name"]),
			Color:       asString(one["color"]),
			Description: asString(one["description"]),
		}
		if o.ID == "" {
			o.ID = f.id("PVTSSO")
		}
		out = append(out, o)
	}
	return out
}

func (f *fakeBoard) indexOf(name string) int {
	for i, field := range f.fields {
		if field.name == name {
			return i
		}
	}
	return -1
}

func (f *fakeBoard) fieldByID(id string) *fakeField {
	for _, field := range f.fields {
		if field.id == id {
			return field
		}
	}
	return nil
}

func (f *fakeBoard) itemByID(id string) *fakeItem {
	for _, item := range f.items {
		if item.id == id {
			return item
		}
	}
	return nil
}

// subject is the issue a comment or a close is aimed at, by its node id.
func (f *fakeBoard) subject(vars map[string]any) (*fakeIssue, error) {
	id := varString(vars, "subject")
	if id == "" {
		id = varString(vars, "issue")
	}
	number, err := strconv.Atoi(strings.TrimPrefix(id, "I_"))
	if err != nil {
		return nil, fmt.Errorf("Could not resolve to a node with the global id of '%s'", id)
	}
	issue := f.issues[number]
	if issue == nil {
		return nil, fmt.Errorf("Could not resolve to a node with the global id of '%s'", id)
	}
	return issue, nil
}

func (f *fakeBoard) inRepo(vars map[string]any) bool {
	return varString(vars, "repoOwner")+"/"+varString(vars, "repoName") == f.repo
}

func (f *fakeBoard) issueID(number int) string {
	return "I_" + strconv.Itoa(number)
}

func optionName(field *fakeField, id string) string {
	for _, o := range field.options {
		if o.ID == id {
			return o.Name
		}
	}
	return ""
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func varString(vars map[string]any, key string) string {
	return asString(vars[key])
}

func varInt(vars map[string]any, key string) int {
	f, _ := vars[key].(float64)
	return int(f)
}

// last is the last request the fake was asked, and how many there were.
func (f *fakeBoard) requests() []asked {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]asked(nil), f.asked...)
}

func (f *fakeBoard) operations() []string {
	var out []string
	for _, a := range f.requests() {
		out = append(out, a.operation)
	}
	return out
}

func (f *fakeBoard) requestCount() int {
	return len(f.requests())
}

func (f *fakeBoard) sleeps() []time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]time.Duration(nil), f.slept...)
}

// tokens is the Authorization header of every request in order, which is how a
// test sees a token being resolved again after a 401.
func (f *fakeBoard) tokens() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.bearer...)
}
