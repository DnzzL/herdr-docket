package basecamp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeTokens struct {
	token string
	err   error
	calls int
}

func (f *fakeTokens) access() (string, error) {
	f.calls++
	return f.token, f.err
}

// newTestAPI points the adapter at a fake Basecamp. Backoff is recorded
// rather than slept so the rate-limit tests finish instantly.
func newTestAPI(t *testing.T, h http.HandlerFunc) (*api, *[]time.Duration) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	slept := &[]time.Duration{}
	return &api{
		base:   srv.URL,
		http:   srv.Client(),
		tokens: &fakeTokens{token: "tok"},
		sleep:  func(d time.Duration) { *slept = append(*slept, d) },
		now:    func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
	}, slept
}

func statusHandler(code int, header map[string]string, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for k, v := range header {
			w.Header().Set(k, v)
		}
		w.WriteHeader(code)
		w.Write([]byte(body))
	}
}

func TestAPISaysWhoItIs(t *testing.T) {
	var got http.Header
	a, _ := newTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.Write([]byte(`{}`))
	})
	if err := a.get("/todos/1.json", nil); err != nil {
		t.Fatal(err)
	}
	if want := "Bearer tok"; got.Get("Authorization") != want {
		t.Errorf("Authorization = %q, want %q", got.Get("Authorization"), want)
	}
	if got.Get("User-Agent") == "" {
		t.Error("no User-Agent: Basecamp refuses requests from clients that do not identify themselves")
	}
}

func TestAPIRetriesAfterTheWaitBasecampAskedFor(t *testing.T) {
	calls := 0
	a, slept := newTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if calls++; calls == 1 {
			statusHandler(http.StatusTooManyRequests, map[string]string{"Retry-After": "7"}, "")(w, r)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	})

	var out struct {
		OK bool `json:"ok"`
	}
	if err := a.get("/todos/1.json", &out); err != nil {
		t.Fatalf("a 429 followed by a 200 should succeed: %v", err)
	}
	if !out.OK {
		t.Error("the retried response body was dropped")
	}
	if len(*slept) != 1 || (*slept)[0] != 7*time.Second {
		t.Fatalf("waited %v, want exactly the 7s Basecamp asked for", *slept)
	}
}

func TestAPINeverSpins(t *testing.T) {
	calls := 0
	a, _ := newTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		statusHandler(http.StatusTooManyRequests, map[string]string{"Retry-After": "1"}, "")(w, r)
	})
	if err := a.get("/todos/1.json", nil); err == nil {
		t.Fatal("a permanently rate-limited API should be an error, not a hang")
	}
	if want := maxRetries + 1; calls != want {
		t.Fatalf("made %d requests, want %d: the retry loop must be bounded", calls, want)
	}
}

func TestAPIRetryAfterMayBeADate(t *testing.T) {
	a, slept := newTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		statusHandler(http.StatusTooManyRequests,
			map[string]string{"Retry-After": "Thu, 01 Jan 2026 00:00:30 GMT"}, "")(w, r)
	})
	a.get("/todos/1.json", nil)
	if len(*slept) == 0 || (*slept)[0] != 30*time.Second {
		t.Fatalf("waited %v, want 30s", *slept)
	}
}

func TestRetryAfterFallsBackWhenItIsMissingOrNonsense(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		header string
		want   time.Duration
	}{
		{"missing", "", time.Second},
		{"not a number or a date", "soon", time.Second},
		{"a date already past", "Thu, 01 Jan 2020 00:00:00 GMT", time.Second},
		{"absurdly long", "86400", maxBackoff},
		{"negative is not a free retry", "-5", time.Second},
		{"zero is not a free retry", "0", time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{Header: http.Header{}}
			if tc.header != "" {
				resp.Header.Set("Retry-After", tc.header)
			}
			if got := retryAfter(resp, now); got != tc.want {
				t.Fatalf("retryAfter(%q) = %v, want %v", tc.header, got, tc.want)
			}
		})
	}
}

func TestAPISaysWhatTheAPIRefused(t *testing.T) {
	a, _ := newTestAPI(t, statusHandler(http.StatusForbidden, nil, `{"error":"bad token"}`))
	err := a.get("/todos/1.json", nil)
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"403", "bad token"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestAPIPostsJSON(t *testing.T) {
	var gotMethod, gotType string
	var gotBody map[string]any
	a, _ := newTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotType = r.Method, r.Header.Get("Content-Type")
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Write([]byte(`{"id":7}`))
	})

	var out struct {
		ID int `json:"id"`
	}
	if err := a.post("/todolists/9/todos.json", map[string]string{"title": "hi"}, &out); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotType != "application/json" || gotBody["title"] != "hi" || out.ID != 7 {
		t.Fatalf("method=%q type=%q body=%v out=%+v", gotMethod, gotType, gotBody, out)
	}
}

func TestAPIAsksForAFreshTokenEveryAttempt(t *testing.T) {
	// A retry after a long wait is exactly when the token may have expired.
	tok := &fakeTokens{token: "tok"}
	srv := httptest.NewServer(statusHandler(http.StatusTooManyRequests,
		map[string]string{"Retry-After": "1"}, ""))
	defer srv.Close()
	a := &api{
		base: srv.URL, http: srv.Client(), tokens: tok,
		sleep: func(time.Duration) {}, now: time.Now,
	}
	a.get("/todos/1.json", nil)
	if want := maxRetries + 1; tok.calls != want {
		t.Fatalf("token asked for %d times, want %d", tok.calls, want)
	}
}
