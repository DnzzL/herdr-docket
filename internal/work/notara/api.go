package notara

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	credstore "github.com/DnzzL/herdr-docket/internal/credentials"
)

// tokenEnv is the first place a key is looked for: the answer for CI, and for
// a machine where the key should not sit in credentials.yaml.
const tokenEnv = "HERDR_DOCKET_NOTARA_TOKEN"

// section is this adapter's block of credentials.yaml.
const section = "notara"

// signIn is the one instruction every missing or refused key ends with.
const signIn = "sign in with `herdr-docket auth notara --token ntr_…` (a key from Notara's sidebar → API keys)"

// requestTimeout bounds one call. Notara is a single server, usually on the
// same machine; a call that has not answered in this long is not going to,
// and a tick must not hang on it.
const requestTimeout = 30 * time.Second

// api is the HTTP half of the adapter: one database's REST paths, bearer
// auth and JSON. Nothing above it knows Notara speaks HTTP.
type api struct {
	base  string // <url>/api/v1/workspaces/<ws>/databases/<db>
	http  *http.Client
	token func() (string, error)
}

// APIError is a call Notara refused. Message is Notara's own words: it names
// the field and the value on a 422, which is the only thing a person can act
// on.
type APIError struct {
	Op      string
	Status  int
	Message string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("notara: %s: %d", e.Op, e.Status)
	if e.Message != "" {
		msg += " " + e.Message
	}
	switch e.Status {
	case http.StatusUnauthorized:
		msg += " — " + signIn
	case http.StatusForbidden:
		msg += " — the key cannot write here (a read key, or no editor access to the database)"
	case http.StatusUnprocessableEntity:
		msg += " — a status or agent the fleet writes must be an option of its field: `herdr-docket auth notara` adds the missing ones"
	}
	return msg
}

func (a *api) do(method, path string, body, out any) error {
	op := method + " " + path
	var payload io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("notara: %s: %v", op, err)
		}
		payload = bytes.NewReader(b)
	}
	token, err := a.token()
	if err != nil {
		return err
	}
	req, err := http.NewRequest(method, a.base+path, payload)
	if err != nil {
		return fmt.Errorf("notara: %s: %v", op, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("notara: %s: %v", op, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &APIError{Op: op, Status: resp.StatusCode, Message: refusal(resp.Body)}
	}
	if out == nil {
		_, err := io.Copy(io.Discard, resp.Body)
		return err
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("notara: %s: decode: %v", op, err)
	}
	return nil
}

// refusal is Notara's `{"error": "…"}`, or whatever else the body says.
func refusal(body io.Reader) string {
	raw, _ := io.ReadAll(io.LimitReader(body, 1024))
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &e) == nil && e.Error != "" {
		return e.Error
	}
	return strings.TrimSpace(string(raw))
}

// record is the path of one card. The short id is what the fleet holds, and
// Notara takes it wherever a record id goes.
func record(id string) string { return "/records/" + url.PathEscape(id) }

// tokenSource hands out the key: the environment first, then the one
// `auth notara` stored. Read once per Source — the fleet builds a Source per
// tick, so a key rotated in credentials.yaml is picked up on the next one.
type tokenSource struct {
	store credstore.File
	env   func(string) string

	once sync.Once
	tok  string
	err  error
}

func defaultTokens() *tokenSource {
	return &tokenSource{store: credstore.Default(), env: os.Getenv}
}

func (t *tokenSource) access() (string, error) {
	t.once.Do(func() {
		if t.env != nil {
			if tok := strings.TrimSpace(t.env(tokenEnv)); tok != "" {
				t.tok = tok
				return
			}
		}
		t.tok, t.err = t.fromStore()
		if t.err == nil && t.tok == "" {
			t.err = errors.New("notara: no API key — " + signIn)
		}
	})
	return t.tok, t.err
}

func (t *tokenSource) fromStore() (string, error) {
	var s stored
	if err := t.store.Load(section, &s); err != nil {
		return "", fmt.Errorf("notara: reading %s: %v", t.store.Path, err)
	}
	return strings.TrimSpace(s.Token), nil
}

// stored is the block in credentials.yaml.
type stored struct {
	Token string `yaml:"token"`
}
