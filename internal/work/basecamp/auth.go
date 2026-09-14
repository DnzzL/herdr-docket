package basecamp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Token is what Launchpad hands back and what we keep. A Basecamp access
// token lives two weeks; the refresh token is what makes that survivable.
type Token struct {
	AccessToken  string    `yaml:"access_token"`
	RefreshToken string    `yaml:"refresh_token"`
	ExpiresAt    time.Time `yaml:"expires_at"`
}

// Valid reports whether the token is still worth using. The slack means a
// token that would expire mid-flight is refreshed before the request, not
// after the failed one.
func (t Token) Valid(now time.Time) bool {
	return t.AccessToken != "" && now.Add(expirySlack).Before(t.ExpiresAt)
}

const expirySlack = time.Minute

// store is where tokens live between runs.
type store interface {
	load() (Token, error)
	save(Token) error
}

// Launchpad is Basecamp's authorization server: its own host, its own token
// endpoint, and no device flow. A CLI has to run a loopback listener, which
// is what Login does.
const (
	authorizeEndpoint = "https://launchpad.37signals.com/authorization/new"
	tokenEndpoint     = "https://launchpad.37signals.com/authorization/token"
)

// Launchpad issues credentials per application, so the fleet cannot ship a
// usable pair: register an app whose redirect URI is
// http://localhost:8917/callback and export these. They are configuration,
// not fleet.yaml, because they identify the app rather than any one fleet.
const (
	clientIDEnv     = "HERDR_DOCKET_BASECAMP_CLIENT_ID"
	clientSecretEnv = "HERDR_DOCKET_BASECAMP_CLIENT_SECRET"
)

// loginPort is fixed so the redirect URI can be registered once.
const loginPort = 8917

// oauth makes the two calls that involve the authorization server.
type oauth struct {
	clientID     string
	clientSecret string
	http         *http.Client
	now          func() time.Time
	endpoint     string
}

func defaultOAuth() *oauth {
	return &oauth{
		clientID:     os.Getenv(clientIDEnv),
		clientSecret: os.Getenv(clientSecretEnv),
		http:         http.DefaultClient,
		now:          time.Now,
		endpoint:     tokenEndpoint,
	}
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (o *oauth) refresh(refreshToken string) (Token, error) {
	return o.exchange(url.Values{
		"type":          {"refresh"},
		"refresh_token": {refreshToken},
	})
}

func (o *oauth) code(code, redirectURI string) (Token, error) {
	return o.exchange(url.Values{
		"type":         {"web_server"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	})
}

func (o *oauth) exchange(form url.Values) (Token, error) {
	form.Set("client_id", o.clientID)
	if o.clientSecret != "" {
		form.Set("client_secret", o.clientSecret)
	}
	req, err := http.NewRequest(http.MethodPost, o.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := o.http.Do(req)
	if err != nil {
		return Token{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode != http.StatusOK {
		return Token{}, fmt.Errorf("launchpad %s: %s: %s",
			form.Get("type"), resp.Status, strings.TrimSpace(string(body)))
	}
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return Token{}, fmt.Errorf("launchpad %s: %w", form.Get("type"), err)
	}
	if tr.AccessToken == "" {
		return Token{}, fmt.Errorf("launchpad %s: no access token in %s",
			form.Get("type"), strings.TrimSpace(string(body)))
	}
	return Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    o.now().Add(time.Duration(tr.ExpiresIn) * time.Second),
	}, nil
}

func (o *oauth) authorizeURL(redirectURI, state string) string {
	q := url.Values{
		"type":         {"web_server"},
		"client_id":    {o.clientID},
		"redirect_uri": {redirectURI},
		"state":        {state},
	}
	return authorizeEndpoint + "?" + q.Encode()
}

// credentials hands out access tokens, refreshing on the way out. A fleet
// left alone for a month heals itself on the next poll instead of failing
// every one of them.
type credentials struct {
	store store
	oauth *oauth
	now   func() time.Time

	mu     sync.Mutex
	token  Token
	loaded bool
}

func (c *credentials) access() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.loaded {
		t, err := c.store.load()
		if err != nil {
			return "", err
		}
		c.token, c.loaded = t, true
	}
	if c.token.Valid(c.now()) {
		return c.token.AccessToken, nil
	}
	if c.token.RefreshToken == "" {
		return "", fmt.Errorf("no Basecamp credentials: run `herdr-docket auth basecamp`")
	}
	fresh, err := c.oauth.refresh(c.token.RefreshToken)
	if err != nil {
		return "", err
	}
	if fresh.RefreshToken == "" {
		fresh.RefreshToken = c.token.RefreshToken
	}
	if err := c.store.save(fresh); err != nil {
		return "", err
	}
	c.token = fresh
	return fresh.AccessToken, nil
}

// Login runs the authorization-code flow: a loopback listener Launchpad can
// redirect to, the authorization page in the user's browser, and the token
// that comes back gets stored. Every later run reads that token instead.
func Login(out io.Writer) error {
	o := defaultOAuth()
	if o.clientID == "" {
		return fmt.Errorf("no Launchpad client id: register an app and set %s (see the README)", clientIDEnv)
	}
	state, err := randomState()
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", loginPort))
	if err != nil {
		return fmt.Errorf("cannot listen on port %d for the redirect: %w", loginPort, err)
	}
	defer ln.Close()

	redirect := fmt.Sprintf("http://localhost:%d/callback", loginPort)
	authURL := o.authorizeURL(redirect, state)
	fmt.Fprintf(out, "Authorize herdr-docket in Basecamp:\n\n  %s\n\n", authURL)
	openBrowser(authURL)

	code, err := waitForCode(ln, state)
	if err != nil {
		return err
	}
	tok, err := o.code(code, redirect)
	if err != nil {
		return err
	}
	s := defaultStore()
	if err := s.save(tok); err != nil {
		return err
	}
	fmt.Fprintf(out, "Basecamp authorized. Credentials stored in %s\n", s.path)
	return nil
}

// waitForCode serves the one request Launchpad will make, and nothing else.
func waitForCode(ln net.Listener, state string) (string, error) {
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second}
	defer srv.Close()

	codes := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			http.Error(w, "Basecamp refused: "+e, http.StatusBadRequest)
			codes <- ""
			return
		}
		if q.Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "Authorized. You can close this tab.")
		codes <- q.Get("code")
	})
	go srv.Serve(ln)

	select {
	case code := <-codes:
		if code == "" {
			return "", fmt.Errorf("Basecamp did not return an authorization code")
		}
		return code, nil
	case <-time.After(5 * time.Minute):
		return "", fmt.Errorf("timed out waiting for Basecamp to redirect back")
	}
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// openBrowser is a convenience: the URL is printed either way, so a failure
// here costs the user a copy and paste, not the login.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
