package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GraphQLURL is GitHub's one GraphQL endpoint. Everything this adapter does —
// reads and writes — goes through it, including the writes REST would spell
// differently: a board is a GraphQL thing, and half-GraphQL, half-REST would
// be two token scopes and two error shapes to keep straight.
const GraphQLURL = "https://api.github.com/graphql"

// UserAgent identifies the fleet, as Basecamp's does.
const UserAgent = "herdr-docket (+https://github.com/DnzzL/herdr-docket)"

// maxRetries bounds the rate-limit retry loop, and maxBackoff caps a wait.
// Being asked to slow down for an hour is being asked to stop.
const (
	maxRetries = 3
	maxBackoff = 60 * time.Second
)

// tokens supplies an access token. Unlike Basecamp's, it can be told to forget
// what it resolved: a GitHub token has no expiry the fleet can see, so the
// only notice it gets that one was revoked is a 401.
type tokens interface {
	access() (string, error)
	invalidate()
}

// api is the GraphQL half of the adapter: one document in, one decoded
// response out, with the token and GitHub's patience handled here so nothing
// above knows it speaks HTTP at all.
type api struct {
	url    string
	http   *http.Client
	tokens tokens
	// sleep is the backoff, injectable so the rate-limit tests do not wait.
	sleep func(time.Duration)
	now   func() time.Time
}

// request is one GraphQL call.
type request struct {
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables,omitempty"`
	OperationName string         `json:"operationName"`
}

// response is a GraphQL answer. Data stays raw: only the caller knows what
// shape its own document asked for.
type response struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors"`
}

// gqlError is one entry of an answer's errors. GitHub's Type is its own
// taxonomy — NOT_FOUND, RATE_LIMITED, FORBIDDEN — and is kept because it says
// what the message only implies.
type gqlError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (r response) err() error {
	if len(r.Errors) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(r.Errors))
	for _, e := range r.Errors {
		if e.Type == "" {
			msgs = append(msgs, e.Message)
			continue
		}
		msgs = append(msgs, e.Type+": "+e.Message)
	}
	return errors.New(strings.Join(msgs, "; "))
}

// do runs one document and decodes its data.
//
// GraphQL refuses in two ways that both mean no: an HTTP status, and a 200
// whose body carries errors. Both are failures here. A mutation that only
// partly applied comes back as the second kind, so an adapter that trusted the
// status code would believe work it did not do.
func (a *api) do(operation, query string, vars map[string]any, out any) error {
	payload, err := json.Marshal(request{Query: query, Variables: vars, OperationName: operation})
	if err != nil {
		return fmt.Errorf("github: %s: %w", operation, err)
	}
	for attempt := 0; ; attempt++ {
		token, err := a.tokens.access()
		if err != nil {
			return err
		}
		body, resp, err := a.post(payload, token)
		if err != nil {
			return fmt.Errorf("github: %s: %w", operation, err)
		}

		switch {
		case resp.StatusCode == http.StatusUnauthorized:
			// The token stopped being good while the process was running. It
			// is resolved again — the environment, gh and the stored file may
			// all say something the memo does not — and tried once more.
			a.tokens.invalidate()
			if attempt == 0 {
				continue
			}
			return fmt.Errorf("github: %s: %s: run `herdr-docket auth github`", operation, statusText(resp, body))
		case rateLimited(resp, body):
			if attempt >= maxRetries {
				return fmt.Errorf("github: %s: %s, and still rate limited after %d attempts", operation, statusText(resp, body), attempt+1)
			}
			a.sleep(backoff(resp, a.now()))
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return fmt.Errorf("github: %s: %s", operation, statusText(resp, body))
		}
		var answer response
		if err := json.Unmarshal(body, &answer); err != nil {
			return fmt.Errorf("github: %s: decode: %w", operation, err)
		}
		if err := answer.err(); err != nil {
			return fmt.Errorf("github: %s: %w", operation, err)
		}
		if out == nil || len(answer.Data) == 0 || string(answer.Data) == "null" {
			return nil
		}
		if err := json.Unmarshal(answer.Data, out); err != nil {
			return fmt.Errorf("github: %s: decode: %w", operation, err)
		}
		return nil
	}
}

// post sends one request and reads its whole answer, which is what the rate
// limit headers and the error body both need.
func (a *api) post(payload []byte, token string) ([]byte, *http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, a.url, bytes.NewReader(payload))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp, err
	}
	return body, resp, nil
}

// rateLimited reports whether the answer is GitHub asking for patience rather
// than answering. It comes three ways: a 429 for a secondary limit, a 403 with
// the hour's budget spent, and — the one a client that only reads status codes
// takes for an answer — a 200 whose errors say RATE_LIMITED.
func rateLimited(resp *http.Response, body []byte) bool {
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return true
	case resp.StatusCode == http.StatusForbidden && resp.Header.Get("x-ratelimit-remaining") == "0":
		return true
	case resp.StatusCode != http.StatusOK:
		return false
	}
	var answer response
	if err := json.Unmarshal(body, &answer); err != nil {
		return false
	}
	for _, e := range answer.Errors {
		if e.Type == "RATE_LIMITED" {
			return true
		}
	}
	return false
}

// backoff is how long to wait before trying again. GitHub's own answer wins:
// Retry-After when it gave one, otherwise the moment the hour's budget comes
// back. Anything unreadable waits a second — the point is to stop hammering,
// not to wait out an outage.
//
// The arithmetic is Basecamp's, spelled again here rather than shared: the two
// agree on twenty lines of header parsing and on nothing else, and the second
// adapter is early for a package.
func backoff(resp *http.Response, now time.Time) time.Duration {
	if v := strings.TrimSpace(resp.Header.Get("Retry-After")); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			if secs <= 0 {
				return time.Second
			}
			return clamp(time.Duration(secs) * time.Second)
		}
		if when, err := http.ParseTime(v); err == nil {
			if d := when.Sub(now); d > 0 {
				return clamp(d)
			}
		}
	}
	if reset := resp.Header.Get("x-ratelimit-reset"); reset != "" {
		if secs, err := strconv.ParseInt(reset, 10, 64); err == nil {
			if d := time.Unix(secs, 0).Sub(now); d > 0 {
				return clamp(d)
			}
		}
	}
	return time.Second
}

func clamp(d time.Duration) time.Duration {
	if d > maxBackoff {
		return maxBackoff
	}
	return d
}

func defaultHTTP() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// statusText is the status plus whatever GitHub said about it. It explains its
// refusals in a JSON message, and "401 Unauthorized" alone gives a user
// nothing to act on.
func statusText(resp *http.Response, body []byte) string {
	var answer struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &answer); err == nil && strings.TrimSpace(answer.Message) != "" {
		return resp.Status + ": " + answer.Message
	}
	if msg := strings.TrimSpace(string(body)); msg != "" {
		if len(msg) > 200 {
			msg = msg[:200] + "…"
		}
		return resp.Status + ": " + msg
	}
	return resp.Status
}
