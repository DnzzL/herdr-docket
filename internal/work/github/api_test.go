package github

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testAPI is an api pointed at a handler, with the waiting recorded instead of
// done and the number of calls handed back. The clock is fixed so a reset
// header can be spelled as a moment.
func testAPI(t *testing.T, h func(w http.ResponseWriter, r *http.Request, call int)) (*api, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		h(w, r, calls)
	}))
	t.Cleanup(srv.Close)
	return &api{
		url:    srv.URL,
		http:   srv.Client(),
		tokens: fixedToken("tok"),
		sleep:  func(time.Duration) {},
		now:    func() time.Time { return time.Unix(1_700_000_000, 0) },
	}, &calls
}

// A 200 is not an answer. GraphQL reports a refused mutation in the body while
// the status says everything went fine, and an adapter that trusted the status
// would believe work it did not do.
func TestARefusalThatArrivesWithA200IsAFailure(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	f.fail["CreateIssue"] = "Something went wrong while executing your query"

	_, err := f.source(t).Create("wire it", "", "")
	if err == nil || !strings.Contains(err.Error(), "Something went wrong") {
		t.Fatalf("Create said %v, want the server's own words", err)
	}
}

func TestTheRequestCarriesTheFleetAndItsToken(t *testing.T) {
	var got *http.Request
	var body string
	api, _ := testAPI(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		got = r
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		body = string(buf)
		w.Write([]byte(`{"data":{"viewer":{"login":"octocat"}}}`))
	})

	var out struct {
		Viewer struct{ Login string }
	}
	if err := api.do("Viewer", viewerQuery, nil, &out); err != nil {
		t.Fatalf("do: %v", err)
	}
	if got.Method != http.MethodPost {
		t.Errorf("the request is a %s, want POST", got.Method)
	}
	if want := "Bearer tok"; got.Header.Get("Authorization") != want {
		t.Errorf("Authorization is %q, want %q", got.Header.Get("Authorization"), want)
	}
	if got.Header.Get("User-Agent") != UserAgent {
		t.Errorf("User-Agent is %q, want the fleet's own", got.Header.Get("User-Agent"))
	}
	if ct := got.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type is %q, want JSON", ct)
	}
	for _, want := range []string{`"query"`, `"operationName":"Viewer"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the request body has no %s: %s", want, body)
		}
	}
}

// GitHub asks for patience in three shapes. All three have to be waited out,
// because all three mean the task was not done.
func TestASecondaryRateLimitIsWaitedOut(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	f.add("wait for me", "")
	f.throttled = 1 // and its Retry-After is two seconds

	if _, err := f.source(t).List(); err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := f.sleeps(); len(got) != 1 || got[0] != 2*time.Second {
		t.Errorf("waited %v, want the one Retry-After GitHub asked for", got)
	}
}

func TestAnHourlyBudgetThatIsSpentIsWaitedOut(t *testing.T) {
	reset := time.Unix(1_700_000_000, 0).Add(30 * time.Second)
	api, _ := testAPI(t, func(w http.ResponseWriter, r *http.Request, call int) {
		if call == 1 {
			w.Header().Set("x-ratelimit-remaining", "0")
			w.Header().Set("x-ratelimit-reset", strconv.FormatInt(reset.Unix(), 10))
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"message":"API rate limit exceeded"}`))
			return
		}
		w.Write([]byte(`{"data":{"viewer":{"login":"octocat"}}}`))
	})
	waited := time.Duration(0)
	api.sleep = func(d time.Duration) { waited = d }

	var out struct {
		Viewer struct{ Login string }
	}
	if err := api.do("Viewer", viewerQuery, nil, &out); err != nil {
		t.Fatalf("do: %v", err)
	}
	if waited != 30*time.Second {
		t.Errorf("waited %v, want the 30s until the budget comes back", waited)
	}
}

func TestARateLimitThatArrivesWithA200IsWaitedOutToo(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	f.limited = true

	if _, err := f.source(t).List(); err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := f.sleeps(); len(got) != 1 {
		t.Errorf("waited %v, want one wait", got)
	}
}

func TestGivingUpOnTheRateLimitSaysSo(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	f.throttled = 99

	_, err := f.source(t).List()
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("List said %v, want it to report the limit it kept hitting", err)
	}
	if got := f.requestCount(); got != maxRetries+1 {
		t.Errorf("tried %d times, want %d", got, maxRetries+1)
	}
	if got := len(f.sleeps()); got != maxRetries {
		t.Errorf("waited %d times, want %d", got, maxRetries)
	}
}

// The one notice this adapter gets that a token was revoked is a 401. It is
// resolved again — gh may have rotated it since — and tried once more, rather
// than reported as a broken board.
func TestAnExpiredTokenIsResolvedAgain(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	f.add("still here", "")
	f.unauthorized = 1

	src := f.source(t)
	resolved := 0
	src.api.tokens = &tokenSource{
		env: func(string) string {
			resolved++
			if resolved == 1 {
				return "stale"
			}
			return "fresh"
		},
		gh: func() (string, error) { return "", nil },
	}

	if _, err := src.List(); err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"Bearer stale", "Bearer fresh"}
	got := f.tokens()
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("the token sent was %v, want %v", got, want)
	}
}

func TestATokenThatStaysBadIsReportedAsASignInProblem(t *testing.T) {
	f := newFakeBoard()
	f.provisionAgent("dev")
	f.add("out of reach", "")
	f.unauthorized = 2

	_, err := f.source(t).List()
	if err == nil || !strings.Contains(err.Error(), "auth github") {
		t.Fatalf("List said %v, want it to point at signing in again", err)
	}
}

func TestAnAnswerThatIsNotJSONIsAFailure(t *testing.T) {
	api, _ := testAPI(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		w.Write([]byte("<html>we are down</html>"))
	})
	if err := api.do("Viewer", viewerQuery, nil, nil); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("do said %v, want a decode failure", err)
	}
}

func TestAnErrorThatIsNotARateLimitIsNotRetried(t *testing.T) {
	api, calls := testAPI(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"Server Error"}`))
	})
	err := api.do("Viewer", viewerQuery, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "Server Error") {
		t.Fatalf("do said %v, want GitHub's message", err)
	}
	if *calls != 1 {
		t.Errorf("tried %d times, want 1: only a rate limit is worth waiting out", *calls)
	}
}

// What a person reads when GitHub refuses. "401 Unauthorized" on its own
// gives them nothing to act on.
func TestARefusalIsExplainedWithWhateverGitHubSaid(t *testing.T) {
	resp := func(status string) *http.Response {
		return &http.Response{Status: status}
	}
	for name, c := range map[string]struct {
		resp *http.Response
		body string
		want string
	}{
		"a message":      {resp("401 Unauthorized"), `{"message":"Bad credentials"}`, "401 Unauthorized: Bad credentials"},
		"a bare body":    {resp("502 Bad Gateway"), "<html>bad gateway</html>", "502 Bad Gateway: <html>bad gateway</html>"},
		"nothing at all": {resp("504 Gateway Timeout"), "", "504 Gateway Timeout"},
		"an essay":       {resp("400 Bad Request"), strings.Repeat("x", 300), "400 Bad Request: " + strings.Repeat("x", 200) + "…"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := statusText(c.resp, []byte(c.body)); got != c.want {
				t.Errorf("statusText = %q, want %q", got, c.want)
			}
		})
	}
}

// backoff is small enough to test directly, and worth it: it is the difference
// between waiting out a limit and hammering an API that is already refusing.
func TestBackoffReadsWhatGitHubSaid(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	answer := func(headers map[string]string) *http.Response {
		resp := &http.Response{Header: http.Header{}, StatusCode: http.StatusForbidden}
		for k, v := range headers {
			resp.Header.Set(k, v)
		}
		return resp
	}
	for name, c := range map[string]struct {
		headers map[string]string
		want    time.Duration
	}{
		"seconds":             {map[string]string{"Retry-After": "5"}, 5 * time.Second},
		"a date":              {map[string]string{"Retry-After": now.Add(20 * time.Second).UTC().Format(http.TimeFormat)}, 20 * time.Second},
		"a date long past":    {map[string]string{"Retry-After": now.Add(-time.Hour).UTC().Format(http.TimeFormat)}, time.Second},
		"nonsense":            {map[string]string{"Retry-After": "soon"}, time.Second},
		"zero":                {map[string]string{"Retry-After": "0"}, time.Second},
		"the hour's reset":    {map[string]string{"x-ratelimit-reset": strconv.FormatInt(now.Add(30*time.Second).Unix(), 10)}, 30 * time.Second},
		"a reset in the past": {map[string]string{"x-ratelimit-reset": strconv.FormatInt(now.Add(-time.Minute).Unix(), 10)}, time.Second},
		"nothing at all":      {nil, time.Second},
		"an hour of it":       {map[string]string{"Retry-After": "3600"}, maxBackoff},
	} {
		t.Run(name, func(t *testing.T) {
			if got := backoff(answer(c.headers), now); got != c.want {
				t.Errorf("backoff = %v, want %v", got, c.want)
			}
		})
	}
}
