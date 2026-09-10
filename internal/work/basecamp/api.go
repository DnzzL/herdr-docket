package basecamp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// APIHost is Basecamp 3's API host. There is no /api/v1 in the path — the
// account id goes straight after the host. (Basecamp 2 lived at
// basecampapi.com and is not compatible with any of this.)
const APIHost = "https://3.basecampapi.com"

// UserAgent identifies the fleet to Basecamp, which rejects requests from
// clients that do not say who they are.
const UserAgent = "herdr-fleet (+https://github.com/DnzzL/herdr-fleet)"

// maxRetries bounds the rate-limit retry loop. Basecamp asks for patience
// with Retry-After; it never asks for forever.
const maxRetries = 3

// maxBackoff caps a Retry-After we would otherwise honour. Being asked to
// slow down for an hour is being asked to stop.
const maxBackoff = 60 * time.Second

// tokens supplies an access token, refreshing the stored one when it has
// expired.
type tokens interface {
	access() (string, error)
}

// api is the HTTP half of the adapter: bearer auth, JSON, and rate limiting.
// Nothing above it knows Basecamp speaks HTTP at all.
type api struct {
	base   string // https://3.basecampapi.com/<account>
	http   *http.Client
	tokens tokens
	// sleep is the backoff, injectable so the rate-limit tests do not wait.
	sleep func(time.Duration)
	now   func() time.Time
}

func (a *api) get(path string, out any) error {
	return a.do(http.MethodGet, path, nil, out)
}

func (a *api) post(path string, body, out any) error {
	return a.do(http.MethodPost, path, body, out)
}

func (a *api) do(method, path string, body, out any) error {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("%s %s: %w", method, path, err)
		}
	}
	for attempt := 0; ; attempt++ {
		token, err := a.tokens.access()
		if err != nil {
			return err
		}
		req, err := http.NewRequest(method, a.base+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("User-Agent", UserAgent)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := a.http.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries {
			wait := retryAfter(resp, a.now())
			resp.Body.Close()
			a.sleep(wait)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			defer resp.Body.Close()
			return fmt.Errorf("%s %s: %s", method, path, statusText(resp))
		}
		if out == nil {
			defer resp.Body.Close()
			_, err := io.Copy(io.Discard, resp.Body)
			return err
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("%s %s: decode: %w", method, path, err)
		}
		return nil
	}
}

// retryAfter reads Basecamp's Retry-After, which may be a number of seconds
// or an HTTP date. Anything unreadable or absurd falls back to a short wait:
// the point is to stop hammering, not to wait out an outage.
func retryAfter(resp *http.Response, now time.Time) time.Duration {
	v := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if secs, err := strconv.Atoi(v); err == nil {
		// A zero or negative wait is not a wait. Honouring it would make the
		// retry loop a spin loop, which is the whole thing being avoided.
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
	// Missing, unreadable, or already past: wait a beat and try again.
	return time.Second
}

func clamp(d time.Duration) time.Duration {
	if d > maxBackoff {
		return maxBackoff
	}
	return d
}

// statusText is the status plus whatever Basecamp said about it. It explains
// its refusals in the body, and "403 Forbidden" alone gives a user nothing to
// act on.
func statusText(resp *http.Response) string {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if msg := strings.TrimSpace(string(body)); msg != "" {
		return resp.Status + ": " + msg
	}
	return resp.Status
}
