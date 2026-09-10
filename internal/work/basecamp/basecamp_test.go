package basecamp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/work"
)

// fakeBasecamp answers the endpoints the adapter uses and remembers what it
// was asked, so a test can assert on the request as well as the response.
type fakeBasecamp struct {
	routes   map[string]string // "METHOD /path" -> body
	requests []string
	bodies   []string
}

func (f *fakeBasecamp) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		f.bodies = append(f.bodies, string(body))
		payload, ok := f.routes[r.Method+" "+r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"Not found"}`))
			return
		}
		w.Write([]byte(payload))
	}
}

// newTestSource builds the real Source from the real config, then swaps the
// two things that would leave the machine: the host and the token. The config
// validation under test is therefore the real validation.
func newTestSource(t *testing.T, cfg Config, f *fakeBasecamp) *Source {
	t.Helper()
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	s.api.base = srv.URL
	s.api.http = srv.Client()
	s.api.tokens = &fakeTokens{token: "tok"}
	s.api.sleep = func(time.Duration) {}
	return s
}

func testConfig() Config {
	return Config{AccountID: "999", Lists: map[string]string{"dev": "111", "pm": "222"}}
}

func TestNewPointsAtTheConfiguredAccount(t *testing.T) {
	s, err := New(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if want := APIHost + "/999"; s.api.base != want {
		t.Fatalf("base = %q, want %q", s.api.base, want)
	}
}

func TestNewRefusesAConfigItCannotRoute(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"no account", Config{Lists: map[string]string{"dev": "111"}}},
		{"no lists at all", Config{AccountID: "999"}},
		{"a list with no id", Config{AccountID: "999", Lists: map[string]string{"dev": "  "}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.cfg); err == nil {
				t.Fatal("want an error: a fleet that cannot be routed can only be misrouted")
			}
		})
	}
}

func TestListReadsOneListPerAgentAndSaysWhoCameFromWhere(t *testing.T) {
	f := &fakeBasecamp{routes: map[string]string{
		"GET /todolists/111/todos.json": `[{"id":1,"title":"dev work","status":"active"}]`,
		"GET /todolists/222/todos.json": `[{"id":2,"title":"pm work","status":"active"}]`,
	}}
	items, err := newTestSource(t, testConfig(), f).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want one from each list", len(items))
	}
	if items[0].ID != "1" || items[0].Assignee != "dev" || items[0].Title != "dev work" {
		t.Errorf("first item = %+v, want dev's to-do", items[0])
	}
	if items[1].ID != "2" || items[1].Assignee != "pm" {
		t.Errorf("second item = %+v, want pm's to-do", items[1])
	}
}

// Basecamp says a to-do is done or it is not. The fleet's board needs words
// for those two conditions, and that translation is the adapter's job.
func TestListMapsBasecampOntoTheFleetsVocabulary(t *testing.T) {
	f := &fakeBasecamp{routes: map[string]string{
		"GET /todolists/111/todos.json": `[
			{"id":1,"title":"open","status":"active"},
			{"id":2,"title":"ticked off","status":"completed","completed":true},
			{"id":3,"title":"archived away","status":"archived"}
		]`,
		"GET /todolists/222/todos.json": `[]`,
	}}
	items, err := newTestSource(t, testConfig(), f).List()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		i     int
		open  bool
		phase string
	}{
		{0, true, "To Do"},
		{1, false, "Done"},
		{2, false, "Done"}, // an archived to-do left the list; it is not owed work
	} {
		if got := items[tc.i]; got.Open != tc.open || got.Phase != tc.phase {
			t.Errorf("%q: Open=%v Phase=%q, want %v/%q", got.Title, got.Open, got.Phase, tc.open, tc.phase)
		}
	}
	if items[0].Verdict != "" {
		t.Errorf("Verdict = %q: Basecamp cannot say how an item ended, so the adapter must not guess", items[0].Verdict)
	}
}

func TestListNamesTheAgentWhoseListFailed(t *testing.T) {
	f := &fakeBasecamp{routes: map[string]string{
		// dev's list (111) is deliberately missing: it 404s.
		"GET /todolists/222/todos.json": `[]`,
	}}
	_, err := newTestSource(t, testConfig(), f).List()
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "dev") {
		t.Errorf("error %q does not say whose list could not be read", err)
	}
}

func TestGetReturnsTheBodyAsMarkdownNotHTML(t *testing.T) {
	f := &fakeBasecamp{routes: map[string]string{
		"GET /todos/7.json": `{"id":7,"title":"t","status":"active","parent":{"id":111,"type":"Todolist"},
			"content":"<div>Do <strong>this</strong> first</div>"}`,
		"GET /recordings/7/comments.json": `[]`,
	}}
	it, err := newTestSource(t, testConfig(), f).Get("7")
	if err != nil {
		t.Fatal(err)
	}
	if it.Body != "Do **this** first" {
		t.Fatalf("Body = %q, want the rich text as markdown", it.Body)
	}
	if strings.ContainsAny(it.Body, "<>") {
		t.Fatalf("Body = %q: HTML must be converted, not passed through", it.Body)
	}
}

// The to-do's list is the routing key, so a to-do read on its own still has
// to come back knowing whose it is.
func TestGetTakesTheAssigneeFromTheListTheTodoIsIn(t *testing.T) {
	f := &fakeBasecamp{routes: map[string]string{
		"GET /todos/7.json":               `{"id":7,"status":"active","parent":{"id":222,"type":"Todolist"}}`,
		"GET /recordings/7/comments.json": `[]`,
	}}
	it, err := newTestSource(t, testConfig(), f).Get("7")
	if err != nil {
		t.Fatal(err)
	}
	if it.Assignee != "pm" {
		t.Fatalf("Assignee = %q, want pm (list 222)", it.Assignee)
	}
}

func TestGetReadsStepsAsCriteria(t *testing.T) {
	f := &fakeBasecamp{routes: map[string]string{
		"GET /todos/7.json": `{"id":7,"status":"active","content":"<div>x</div>","steps":[
			{"id":20,"title":"second","completed":false,"position":2},
			{"id":10,"title":"first","completed":true,"position":1}
		]}`,
		"GET /recordings/7/comments.json": `[]`,
	}}
	it, err := newTestSource(t, testConfig(), f).Get("7")
	if err != nil {
		t.Fatal(err)
	}
	want := []work.Criterion{
		{Index: 1, Text: "first", Checked: true},
		{Index: 2, Text: "second", Checked: false},
	}
	if !reflect.DeepEqual(it.Criteria, want) {
		t.Fatalf("Criteria = %+v, want %+v (in Basecamp's own order)", it.Criteria, want)
	}
}

func TestGetReadsCommentsAsNotes(t *testing.T) {
	f := &fakeBasecamp{routes: map[string]string{
		"GET /todos/7.json": `{"id":7,"status":"active","content":"<div>x</div>"}`,
		"GET /recordings/7/comments.json": `[
			{"id":1,"content":"<div>needs a rebase</div>","creator":{"name":"Ada"}},
			{"id":2,"content":"<div>done now</div>","creator":{"name":"Bob"}}
		]`,
	}}
	it, err := newTestSource(t, testConfig(), f).Get("7")
	if err != nil {
		t.Fatal(err)
	}
	if want := "Ada: needs a rebase\n\nBob: done now"; it.Notes != want {
		t.Fatalf("Notes = %q, want %q", it.Notes, want)
	}
}

// content is what Basecamp sends and description is a plain-text summary it
// truncates in list responses, so the rich text wins whenever there is any.
func TestGetFallsBackToThePlainDescription(t *testing.T) {
	f := &fakeBasecamp{routes: map[string]string{
		"GET /todos/7.json":               `{"id":7,"status":"active","content":"","description":"plain words"}`,
		"GET /recordings/7/comments.json": `[]`,
	}}
	it, err := newTestSource(t, testConfig(), f).Get("7")
	if err != nil {
		t.Fatal(err)
	}
	if it.Body != "plain words" {
		t.Fatalf("Body = %q, want the plain description", it.Body)
	}
}

func TestCreatePostsIntoTheListThatCarriesTheAssignee(t *testing.T) {
	f := &fakeBasecamp{routes: map[string]string{"POST /todolists/222/todos.json": `{"id":42}`}}
	id, err := newTestSource(t, testConfig(), f).Create("a title", "a body", "pm")
	if err != nil {
		t.Fatal(err)
	}
	if id != "42" {
		t.Fatalf("id = %q, want the new to-do's id", id)
	}
	if want := []string{"POST /todolists/222/todos.json"}; !reflect.DeepEqual(f.requests, want) {
		t.Fatalf("requests = %v, want %v", f.requests, want)
	}
	var sent content
	if err := json.Unmarshal([]byte(f.bodies[0]), &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Title != "a title" || !strings.Contains(sent.Content, "a body") {
		t.Fatalf("posted %+v, want the title and the body", sent)
	}
}

// Without a list there is nowhere to put the work, and guessing would hand it
// to the wrong agent. So it is an error, and Basecamp is not even called.
func TestCreateRefusesAnAgentWithNoList(t *testing.T) {
	f := &fakeBasecamp{}
	_, err := newTestSource(t, testConfig(), f).Create("t", "b", "ghost")
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error %q does not name the agent it cannot route", err)
	}
	if len(f.requests) != 0 {
		t.Fatalf("called Basecamp anyway: %v", f.requests)
	}
}

func TestCommentPostsRichText(t *testing.T) {
	f := &fakeBasecamp{routes: map[string]string{"POST /recordings/7/comments.json": `{}`}}
	if err := newTestSource(t, testConfig(), f).Comment("7", "two\nlines"); err != nil {
		t.Fatal(err)
	}
	var sent content
	if err := json.Unmarshal([]byte(f.bodies[0]), &sent); err != nil {
		t.Fatal(err)
	}
	if want := "<div>two<br>lines</div>"; sent.Content != want {
		t.Fatalf("comment = %q, want %q", sent.Content, want)
	}
}

// Basecamp has one word for an ending, so a failed or blocked item would
// otherwise be indistinguishable from a finished one. The comment is the only
// place left to say which it was.
func TestCloseCompletesTheTodoThenRecordsTheVerdict(t *testing.T) {
	for _, v := range []work.Verdict{work.Done, work.Failed, work.Blocked} {
		t.Run(string(v), func(t *testing.T) {
			f := &fakeBasecamp{routes: map[string]string{
				"POST /todos/7/completion.json":    `{}`,
				"POST /recordings/7/comments.json": `{}`,
			}}
			if err := newTestSource(t, testConfig(), f).Close("7", v); err != nil {
				t.Fatal(err)
			}
			want := []string{"POST /todos/7/completion.json", "POST /recordings/7/comments.json"}
			if !reflect.DeepEqual(f.requests, want) {
				t.Fatalf("requests = %v, want %v", f.requests, want)
			}
			var sent content
			if err := json.Unmarshal([]byte(f.bodies[1]), &sent); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(sent.Content, string(v)) {
				t.Fatalf("comment %q does not say how the item ended", sent.Content)
			}
		})
	}
}

// The fleet writes prose, not markup: a body with angle brackets in it must
// arrive as those characters, not as a tag Basecamp's editor would run with.
func TestABodyIsEscapedRatherThanTurnedIntoMarkup(t *testing.T) {
	got := richText("a <b>bold</b> claim")
	if !strings.Contains(got, "&lt;b&gt;") || strings.Contains(got, "<b>") {
		t.Fatalf("richText(%q) = %q, want the angle brackets escaped", "a <b>bold</b> claim", got)
	}
}
