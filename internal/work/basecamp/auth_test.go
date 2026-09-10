package basecamp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	token Token
	saved []Token
	err   error
}

func (f *fakeStore) load() (Token, error) { return f.token, f.err }

func (f *fakeStore) save(t Token) error {
	f.saved = append(f.saved, t)
	f.token = t
	return nil
}

func fixedNow() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }

// launchpadFake answers the token endpoint with a token that lives two weeks,
// which is what Launchpad actually sends.
func launchpadFake(t *testing.T) (*oauth, *[]string) {
	t.Helper()
	forms := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*forms = append(*forms, string(body))
		w.Write([]byte(`{"access_token":"fresh","refresh_token":"newref","expires_in":1209600}`))
	}))
	t.Cleanup(srv.Close)
	return &oauth{
		clientID: "cid", clientSecret: "sec", http: srv.Client(),
		now: fixedNow, endpoint: srv.URL,
	}, forms
}

func TestAValidTokenIsUsedWithoutAskingLaunchpad(t *testing.T) {
	o, forms := launchpadFake(t)
	st := &fakeStore{token: Token{AccessToken: "good", RefreshToken: "ref", ExpiresAt: fixedNow().Add(time.Hour)}}
	c := &credentials{store: st, oauth: o, now: fixedNow}

	got, err := c.access()
	if err != nil {
		t.Fatal(err)
	}
	if got != "good" {
		t.Errorf("access = %q, want the stored token", got)
	}
	if len(*forms) != 0 {
		t.Errorf("refreshed a token that was still valid: %v", *forms)
	}
}

func TestAnExpiredTokenIsRefreshedAndStored(t *testing.T) {
	o, forms := launchpadFake(t)
	st := &fakeStore{token: Token{AccessToken: "stale", RefreshToken: "ref", ExpiresAt: fixedNow().Add(-time.Hour)}}
	c := &credentials{store: st, oauth: o, now: fixedNow}

	got, err := c.access()
	if err != nil {
		t.Fatal(err)
	}
	if got != "fresh" {
		t.Errorf("access = %q, want the refreshed token", got)
	}
	if len(st.saved) != 1 {
		t.Fatalf("stored %d tokens, want 1: an unexpired refresh must survive the process", len(st.saved))
	}
	if st.saved[0].AccessToken != "fresh" || !st.saved[0].ExpiresAt.After(fixedNow()) {
		t.Errorf("stored %+v, want the fresh token with its new expiry", st.saved[0])
	}
	if len(*forms) != 1 || !strings.Contains((*forms)[0], "type=refresh") || !strings.Contains((*forms)[0], "refresh_token=ref") {
		t.Errorf("refresh form = %v, want a refresh grant carrying the refresh token", *forms)
	}
	if !strings.Contains((*forms)[0], "client_id=cid") {
		t.Errorf("refresh form %q does not identify the application", (*forms)[0])
	}
}

// A refresh that returns no new refresh token means keep the old one: losing
// it locks the fleet out until someone logs in by hand.
func TestRefreshKeepsTheOldRefreshTokenWhenLaunchpadSendsANewPairOnlyOnce(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"access_token":"fresh","expires_in":1209600}`))
	}))
	defer srv.Close()
	o := &oauth{clientID: "cid", http: srv.Client(), now: fixedNow, endpoint: srv.URL}
	st := &fakeStore{token: Token{AccessToken: "stale", RefreshToken: "ref", ExpiresAt: fixedNow().Add(-time.Hour)}}

	if _, err := (&credentials{store: st, oauth: o, now: fixedNow}).access(); err != nil {
		t.Fatal(err)
	}
	if st.saved[0].RefreshToken != "ref" {
		t.Errorf("stored refresh token %q, want the one that was already there", st.saved[0].RefreshToken)
	}
}

// The fresh token is asked for once, not once per call: a fleet polls often.
func TestARefreshedTokenIsReusedForTheRestOfTheProcess(t *testing.T) {
	o, forms := launchpadFake(t)
	st := &fakeStore{token: Token{AccessToken: "stale", RefreshToken: "ref", ExpiresAt: fixedNow().Add(-time.Hour)}}
	c := &credentials{store: st, oauth: o, now: fixedNow}

	for range 3 {
		if _, err := c.access(); err != nil {
			t.Fatal(err)
		}
	}
	if len(*forms) != 1 {
		t.Fatalf("asked Launchpad %d times, want 1", len(*forms))
	}
}

func TestNoCredentialsSayWhatToRun(t *testing.T) {
	o, _ := launchpadFake(t)
	err := func() error {
		_, err := (&credentials{store: &fakeStore{}, oauth: o, now: fixedNow}).access()
		return err
	}()
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "auth basecamp") {
		t.Errorf("error %q does not tell the user which command to run", err)
	}
}

func TestALoadFailureIsNotMistakenForNoToken(t *testing.T) {
	o, _ := launchpadFake(t)
	st := &fakeStore{err: io.ErrUnexpectedEOF}
	err := func() error { _, err := (&credentials{store: st, oauth: o, now: fixedNow}).access(); return err }()
	if err == nil || strings.Contains(err.Error(), "auth basecamp") {
		t.Fatalf("error = %v, want the real read failure", err)
	}
}

func TestAuthorizeURLAsksForTheCodeFlowTheFleetCanRun(t *testing.T) {
	o := &oauth{clientID: "cid"}
	got := o.authorizeURL("http://localhost:8917/callback", "xyz")
	for _, want := range []string{"launchpad.37signals.com", "type=web_server", "client_id=cid", "state=xyz"} {
		if !strings.Contains(got, want) {
			t.Errorf("authorize URL %q is missing %q", got, want)
		}
	}
}
