package github

import (
	"strconv"
	"strings"
	"testing"

	"github.com/DnzzL/herdr-docket/internal/work"
	"github.com/DnzzL/herdr-docket/internal/work/worktest"
)

// The contract every source must keep, against a backend it can empty between
// subtests. An adapter that only passes this is still an adapter with no
// opinion; the tests below it are where the board's own behaviour lives.
func TestSourceKeepsTheContract(t *testing.T) {
	worktest.Run(t, func(t *testing.T) work.Source { return newSource(t) })
}

func TestListReadsOnePageOfTheBoardInItsOwnOrder(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	first := f.add("first", "")
	second := f.add("second", "")
	f.addDraft() // a card on the board that is not an issue

	items, err := f.source(t).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("List returned %d tasks, want 2: %+v", len(items), items)
	}
	for i, want := range []struct {
		number int
		title  string
	}{{first, "first"}, {second, "second"}} {
		got := items[i]
		if got.ID != strconv.Itoa(want.number) {
			t.Errorf("task %d is %q, want the issue number %d", i, got.ID, want.number)
		}
		if got.Title != want.title {
			t.Errorf("task %d is %q, want %q", i, got.Title, want.title)
		}
		if got.Ordinal != float64(i) {
			t.Errorf("task %d has ordinal %v, want the board's position %d", i, got.Ordinal, i)
		}
		if !got.Open {
			t.Errorf("task %d is not open", i)
		}
	}

	if n := f.requestCount(); n != 1 {
		t.Errorf("List took %d requests, want 1", n)
	}
}

// The queue is one page of a search, and the search is what makes a page of a
// board into a queue of one repository's issues.
func TestListNarrowsTheSearchOnTheServer(t *testing.T) {
	f := newFakeBoard()
	f.source(t).List() //nolint:errcheck // the shape of the request is the point

	request := f.requests()[0]
	if got := varString(request.vars, "filter"); got != "repo:acme/widgets is:issue" {
		t.Errorf("the search filter is %q, want one repository's issues", got)
	}
	if got := varString(request.vars, "agentField"); got != "Agent" {
		t.Errorf("the routing field asked for is %q, want Agent", got)
	}
}

// Points are charged per hundred objects returned, and List runs on a timer.
// A body or a thread per item would cost about a hundred points a tick.
func TestListAsksForNothingTheQueueDoesNotRead(t *testing.T) {
	f := newFakeBoard()
	f.source(t).List() //nolint:errcheck // the shape of the request is the point

	query := f.requests()[0].query
	for _, want := range []string{"number", "title", "state", "createdAt"} {
		if !strings.Contains(query, want) {
			t.Errorf("the queue does not ask for %q:\n%s", want, query)
		}
	}
	for _, unwanted := range []string{"body", "comments", "notes"} {
		if strings.Contains(query, unwanted) {
			t.Errorf("the queue asks for %q, which the poll must not pay for:\n%s", unwanted, query)
		}
	}
}

// A verdict is not in the page — the board cannot say how a task ended without
// its thread — so the poll believes a column only when it names an ending.
func TestListShowsAClosedTaskOnlyByItsColumn(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	done := f.add("done", "")
	f.end(done) // closed, and GitHub's automation moved it to Done
	lost := f.add("lost", "")
	f.end(lost)
	f.setStatus(lost, "Todo") // closed, and the column says otherwise

	items, err := f.source(t).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	byID := map[string]work.Task{}
	for _, it := range items {
		byID[it.ID] = it
	}
	if got := byID[strconv.Itoa(done)]; got.Open || got.Phase != "Done" {
		t.Errorf("a closed task in Done reads open=%v phase=%q, want closed and Done", got.Open, got.Phase)
	}
	if got := byID[strconv.Itoa(lost)]; got.Open || got.Phase != "" {
		t.Errorf("a closed task whose column is not an ending reads open=%v phase=%q, want closed and no phase", got.Open, got.Phase)
	}
}

// The column describes and the issue's state decides: a task somebody moved
// into Done without closing it is still work the fleet will run.
func TestAnOpenTaskInAFinishedColumnIsStillOpen(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	n := f.add("not done yet", "")
	f.setStatus(n, "Done")

	items, err := f.source(t).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || !items[0].Open {
		t.Fatalf("a task in Done but not closed must stay in the queue: %+v", items)
	}
}

func TestGetReadsTheBodyTheChecklistAndTheThread(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	n := f.add("Wire the gauge", "Notes:\n\n- [x] read the sensor\n- [ ] wire the display\n  - [ ] and the buzzer\n\nnothing follows")
	f.issue(n).comments = []gqlComment{
		{Body: "started", Author: gqlAuthor{Login: "octocat"}},
		{Body: "halfway", Author: gqlAuthor{Login: "dev"}},
	}
	f.setStatus(n, "In Progress")

	task, err := f.source(t).Get(strconv.Itoa(n))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if task.Title != "Wire the gauge" {
		t.Errorf("title = %q", task.Title)
	}
	if !strings.Contains(task.Body, "read the sensor") {
		t.Errorf("body lost the description: %q", task.Body)
	}
	if task.Phase != "In Progress" {
		t.Errorf("phase = %q, want the board's column", task.Phase)
	}
	want := []work.Criterion{
		{Index: 1, Text: "read the sensor", Checked: true},
		{Index: 2, Text: "wire the display"},
		{Index: 3, Text: "and the buzzer"},
	}
	if len(task.Criteria) != len(want) {
		t.Fatalf("criteria = %+v, want %d of them", task.Criteria, len(want))
	}
	for i, c := range want {
		if task.Criteria[i] != c {
			t.Errorf("criterion %d = %+v, want %+v", i+1, task.Criteria[i], c)
		}
	}
	if !strings.Contains(task.Notes, "octocat: started") || !strings.Contains(task.Notes, "dev: halfway") {
		t.Errorf("notes lost the thread or its authors: %q", task.Notes)
	}
}

// The whole reason a verdict is a comment: the issue's own state cannot say
// how the work went, and the fleet's board must still show it.
func TestGetReadsTheVerdictOfAClosedTask(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	n := f.addClosed("given up on", "", "blocked")

	task, err := f.source(t).Get(strconv.Itoa(n))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if task.Open {
		t.Error("a closed issue must read as closed")
	}
	if task.Verdict != work.Blocked {
		t.Errorf("verdict = %q, want blocked", task.Verdict)
	}
	if task.Phase != "Blocked" {
		t.Errorf("phase = %q, want the verdict's own word", task.Phase)
	}
}

// An open task carrying an old verdict is work a human has since unblocked.
// Reading it back would trap the task in the phase it was rescued from.
func TestAVerdictIsIgnoredWhileTheTaskIsOpen(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	n := f.add("tried again", "")
	f.issue(n).comments = []gqlComment{{Body: verdictPrefix + "blocked", Author: gqlAuthor{Login: testViewer}}}
	f.setStatus(n, "Todo")

	task, err := f.source(t).Get(strconv.Itoa(n))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if task.Verdict != "" {
		t.Errorf("verdict = %q, want nothing: the task is open", task.Verdict)
	}
	if task.Phase != "To Do" {
		t.Errorf("phase = %q, want the board's column", task.Phase)
	}
}

// Closing, unblocking and closing again is the loop that makes a blocked task
// recoverable. The second close must win, so the newest verdict is the one
// read.
func TestClosingAgainAfterAReopenReportsTheNewVerdict(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	n := f.add("blocked then done", "")
	src := f.source(t)
	id := strconv.Itoa(n)

	if err := src.Close(id, work.Blocked); err != nil {
		t.Fatalf("Close(blocked): %v", err)
	}
	if task, err := src.Get(id); err != nil || task.Verdict != work.Blocked {
		t.Fatalf("after Close(blocked): verdict = %q, err = %v", task.Verdict, err)
	}

	f.reopen(n)
	task, err := src.Get(id)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if !task.Open || task.Verdict != "" {
		t.Errorf("a reopened task reads open=%v verdict=%q, want open and no verdict", task.Open, task.Verdict)
	}

	if err := src.Close(id, work.Done); err != nil {
		t.Fatalf("Close(done): %v", err)
	}
	task, err = src.Get(id)
	if err != nil {
		t.Fatalf("Get after the second close: %v", err)
	}
	if task.Verdict != work.Done {
		t.Errorf("verdict = %q, want the newest thing said, done", task.Verdict)
	}
}

// An issue in the repository that nobody put on the board is not this fleet's
// work. Saying so is the whole answer: quietly adding it would be the fleet
// claiming a list it was never given.
func TestAnIssueThatIsNotOnTheBoardIsNotOurWork(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	loose := f.addUnboarded("nobody's queue")

	src := f.source(t)
	id := strconv.Itoa(loose)
	if _, err := src.Get(id); err == nil || !strings.Contains(err.Error(), "not on") {
		t.Errorf("Get on an unboarded issue said %v, want it to say the issue is not on the board", err)
	}
	if err := src.Comment(id, "hello"); err == nil || !strings.Contains(err.Error(), "not on") {
		t.Errorf("Comment on an unboarded issue said %v, want it to refuse", err)
	}
	if err := src.Close(id, work.Done); err == nil || !strings.Contains(err.Error(), "not on") {
		t.Errorf("Close on an unboarded issue said %v, want it to refuse", err)
	}
	if err := src.Assign(id, "dev"); err == nil || !strings.Contains(err.Error(), "not on") {
		t.Errorf("Assign on an unboarded issue said %v, want it to refuse", err)
	}
	if f.issue(loose).state != "OPEN" {
		t.Error("a refused close must change nothing")
	}
}

func TestGetOnAMissingIssueSaysWhichRepository(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	_, err := f.source(t).Get("404")
	if err == nil || !strings.Contains(err.Error(), "no issue 404 in acme/widgets") {
		t.Errorf("Get(404) said %v, want it to name the missing issue", err)
	}
}

func TestAnIdThatIsNotAnIssueNumberIsRefused(t *testing.T) {
	f := newFakeBoard()
	src := f.source(t)
	for _, id := range []string{"", "no-such-item-4f2a", "PVTI_x", "42/43"} {
		if _, err := src.Get(id); err == nil {
			t.Errorf("Get(%q) was accepted, want a refusal", id)
		}
	}
	if n := f.requestCount(); n != 0 {
		t.Errorf("%d requests went out for ids that cannot exist, want none", n)
	}
}

// Both mutations are one document, and the close is first: an issue the fleet
// has finished with is one it will not run twice.
func TestCloseEndsTheTaskAndSaysHowItWent(t *testing.T) {
	for _, c := range []struct {
		verdict work.Verdict
		reason  string
	}{
		{work.Done, "COMPLETED"},
		{work.Failed, "NOT_PLANNED"},
		{work.Blocked, "NOT_PLANNED"},
	} {
		t.Run(string(c.verdict), func(t *testing.T) {
			f := newFakeBoard()
			f.provisionAgent("dev")
			n := f.add("finish me", "")

			if err := f.source(t).Close(strconv.Itoa(n), c.verdict); err != nil {
				t.Fatalf("Close(%s): %v", c.verdict, err)
			}
			if got := strings.Join(f.operations(), " "); got != "Target Close" {
				t.Errorf("Close made the requests %v, want one to resolve it and one to end it", got)
			}
			issue := f.issue(n)
			if issue.state != "CLOSED" {
				t.Errorf("the issue is %s, want closed", issue.state)
			}
			if issue.reason != c.reason {
				t.Errorf("the state reason is %q, want %q", issue.reason, c.reason)
			}
			if got := issue.comments[len(issue.comments)-1].Body; got != "Verdict: "+string(c.verdict) {
				t.Errorf("the closing comment is %q, want the verdict", got)
			}
			query := f.requests()[1].query
			if !strings.Contains(query, "closeIssue") || !strings.Contains(query, "addComment") {
				t.Errorf("the close and the verdict must travel in one document:\n%s", query)
			}
			if strings.Index(query, "closeIssue") > strings.Index(query, "addComment") {
				t.Error("the close must come before the comment, so a failure to comment cannot leave the task open")
			}
		})
	}
}

func TestAnUnknownVerdictIsRefusedWithoutWriting(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	n := f.add("not so fast", "")

	err := f.source(t).Close(strconv.Itoa(n), work.Verdict("probably"))
	if err == nil {
		t.Fatal("an unknown verdict must be refused")
	}
	if f.requestCount() != 0 {
		t.Error("a refused verdict must not reach GitHub at all")
	}
	if f.issue(n).state != "OPEN" {
		t.Error("a refused verdict must leave the issue open")
	}
}

// The column is shown to a person watching the board. It is written
// best-effort, and never read back to decide anything.
func TestSetPhaseWritesTheBoardsOwnColumn(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	n := f.add("in hand", "")

	if err := f.source(t).SetPhase(strconv.Itoa(n), work.PhaseInProgress); err != nil {
		t.Fatalf("SetPhase: %v", err)
	}
	if got := f.itemOf(n).status; got != "In Progress" {
		t.Errorf("the board's column is %q, want In Progress", got)
	}
	if n := f.requestCount(); n != 2 {
		t.Errorf("SetPhase took %d requests, want 2 (resolve, then write)", n)
	}
}

func TestSetPhaseSaysSoWhenTheBoardHasNoSuchColumn(t *testing.T) {
	f := newFakeBoard() // a board whose Status is GitHub's default, without In Progress
	f.fields = []*fakeField{f.newField("Status", "Todo", "Done")}
	f.provisionAgent("dev")
	n := f.add("in hand", "")

	err := f.source(t).SetPhase(strconv.Itoa(n), work.PhaseInProgress)
	if err == nil || !strings.Contains(err.Error(), "In Progress") {
		t.Errorf("SetPhase said %v, want it to name the missing column", err)
	}
	if got := f.itemOf(n).status; got != "" {
		t.Errorf("the column was written to %q, want nothing written", got)
	}
}

func TestAssignHandsTheTaskOn(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev", "pm")
	n := f.add("hand me on", "")

	src := f.source(t)
	if err := src.Assign(strconv.Itoa(n), "pm"); err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if got := f.itemOf(n).agent; got != "pm" {
		t.Errorf("the routing key is %q, want pm", got)
	}
	task, err := src.Get(strconv.Itoa(n))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if task.Assignee != "pm" {
		t.Errorf("the task reads as routed to %q, want pm", task.Assignee)
	}
}

// A routing write that cannot be made must fail loudly. Clearing the field
// instead would take the task off every agent's board at once and look like it
// worked.
func TestAssignRefusesAnAgentTheBoardCannotRouteTo(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	n := f.add("mine", "")
	f.itemOf(n).agent = "dev"

	err := f.source(t).Assign(strconv.Itoa(n), "pm")
	if err == nil || !strings.Contains(err.Error(), "not an option") {
		t.Fatalf("Assign(pm) said %v, want it to refuse", err)
	}
	if got := f.itemOf(n).agent; got != "dev" {
		t.Errorf("the routing key is %q after a refused assign, want dev untouched", got)
	}
}

func TestCreateWritesTheIssueOntoTheBoardAndRoutesIt(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	src := f.source(t)

	id, err := src.Create("Wire the gauge", "with a checklist:\n- [ ] once", "dev")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := f.operations(); strings.Join(got, " ") != "Create CreateIssue SetField" {
		t.Errorf("Create made the requests %v, want the routing key checked before anything is written", got)
	}
	number, err := strconv.Atoi(id)
	if err != nil {
		t.Fatalf("Create returned %q, want an issue number", id)
	}
	if item := f.itemOf(number); item == nil {
		t.Fatal("the created issue is not on the board")
	} else if item.agent != "dev" {
		t.Errorf("the created issue is routed to %q, want dev", item.agent)
	}

	items, err := src.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || items[0].ID != id {
		t.Fatalf("List returned %+v, want the created task %s", items, id)
	}
}

// An agent that is not on the board is a mistake in configuration. Discovering
// it after writing an issue would leave work nobody asked for.
func TestCreateRefusesAnUnknownAgentBeforeWritingAnything(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")

	if _, err := f.source(t).Create("wire it", "", "pm"); err == nil || !strings.Contains(err.Error(), "not an option") {
		t.Fatalf("Create(pm) said %v, want it to refuse", err)
	}
	if got := f.operations(); strings.Join(got, " ") != "Create" {
		t.Errorf("Create(pm) made the requests %v, want only the one that reads the board", got)
	}
	if len(f.issues) != 0 {
		t.Errorf("%d issues were written, want none", len(f.issues))
	}
}

func TestCreateWithoutARoutingKeyWritesNoRouting(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")

	id, err := f.source(t).Create("nobody's", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := f.operations(); strings.Join(got, " ") != "Create CreateIssue" {
		t.Errorf("Create made the requests %v, want no routing write", got)
	}
	number, _ := strconv.Atoi(id)
	if got := f.itemOf(number).agent; got != "" {
		t.Errorf("an unrouted issue is on %q, want no routing key", got)
	}
}

// A board can belong to a person as easily as to an organization, and which
// one it is is not something the configuration says.
func TestABoardOwnedByAUserIsFoundTheSameWay(t *testing.T) {
	f := newFakeBoard()
	f.asUser = true
	f.provisionAgent("dev")
	f.add("a person's work", "")

	items, err := f.source(t).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("List returned %+v, want the one task", items)
	}
}

func TestAPageOfABoardThatIsNotThereIsReportedAsSuch(t *testing.T) {
	f := newFakeBoard()
	f.owner = "somebody-else"

	_, err := f.source(t).List()
	if err == nil || !strings.Contains(err.Error(), "no project 3") {
		t.Errorf("List said %v, want it to name the project it could not find", err)
	}
}

func TestNewRefusesAConfigurationItCannotRoute(t *testing.T) {
	for name, c := range map[string]Config{
		"no owner":            {Project: 3, Repo: "acme/widgets"},
		"no project":          {Owner: "acme", Repo: "acme/widgets"},
		"no repository":       {Owner: "acme", Project: 3},
		"a bare repository":   {Owner: "acme", Project: 3, Repo: "widgets"},
		"an odd repository":   {Owner: "acme", Project: 3, Repo: "acme/widgets/extra"},
		"an empty repository": {Owner: "acme", Project: 3, Repo: "acme/"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(c); err == nil {
				t.Errorf("New(%+v) was accepted, want a refusal", c)
			}
		})
	}
}

// The repository is split once, where the configuration is read, so the rule
// about what a repository may look like lives here and not at each document.
func TestARepositoryIsAnOwnerAndAName(t *testing.T) {
	for _, s := range []string{"acme/widgets", "acme/one", "a-b_c/9"} {
		r, err := parseRepo(s)
		if err != nil {
			t.Fatalf("parseRepo(%q): %v", s, err)
		}
		owner, name, _ := strings.Cut(s, "/")
		if r != (repo{owner: owner, name: name}) {
			t.Errorf("parseRepo(%q) = %+v, want %s and %s", s, r, owner, name)
		}
	}
	for _, s := range []string{"", "acme", "/widgets", "acme/", "/", "acme/widgets/extra"} {
		if r, err := parseRepo(s); err == nil {
			t.Errorf("parseRepo(%q) = %+v, want a refusal", s, r)
		}
	}
}
