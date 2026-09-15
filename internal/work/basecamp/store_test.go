package basecamp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func inTempConfigDir(t *testing.T) {
	t.Helper()
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", t.TempDir())
}

func TestCredentialsRoundTrip(t *testing.T) {
	inTempConfigDir(t)
	s := defaultStore()
	want := Token{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
	}
	if err := s.save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("loaded %+v, want %+v", got, want)
	}
}

// A secret file that was once world-readable must not stay that way. WriteFile
// only applies its mode when it creates the file.
func TestSavingCredentialsTightensANameableFile(t *testing.T) {
	inTempConfigDir(t)
	s := defaultStore()
	if err := os.MkdirAll(filepath.Dir(s.file.Path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.file.Path, []byte("basecamp:\n  access_token: leaked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.save(Token{AccessToken: "at"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(s.file.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("credentials.yaml is %o, want 600", got)
	}
}

func TestAMissingCredentialsFileIsNotAnError(t *testing.T) {
	inTempConfigDir(t)
	got, err := defaultStore().load()
	if err != nil {
		t.Fatalf("a fleet that has never logged in should be an empty token, not a failure: %v", err)
	}
	if got.AccessToken != "" {
		t.Fatalf("loaded %+v from nowhere", got)
	}
}

func TestTokenValidityLeavesRoomForTheRoundTrip(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		token Token
		want  bool
	}{
		{"an hour left", Token{AccessToken: "a", ExpiresAt: now.Add(time.Hour)}, true},
		{"expiring while the request is in flight", Token{AccessToken: "a", ExpiresAt: now.Add(30 * time.Second)}, false},
		{"already expired", Token{AccessToken: "a", ExpiresAt: now.Add(-time.Hour)}, false},
		{"no access token at all", Token{ExpiresAt: now.Add(time.Hour)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.token.Valid(now); got != tc.want {
				t.Fatalf("Valid = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLoginRefusesBeforeItListensWithoutAClientID(t *testing.T) {
	inTempConfigDir(t)
	t.Setenv(clientIDEnv, "")
	var out strings.Builder
	err := Login(&out)
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), clientIDEnv) {
		t.Errorf("error %q does not name the variable to set", err)
	}
}
