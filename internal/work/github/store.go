package github

import (
	credstore "github.com/DnzzL/herdr-docket/internal/credentials"
)

// section is this adapter's block of credentials.yaml. The file itself — its
// mode, its other sections, the read-modify-write that keeps them — belongs to
// internal/credentials, because several backends write to it.
const section = "github"

// token is the shape of the block. A GitHub token has nothing to refresh and
// no expiry of its own, so there is one field and no logic around it.
type token struct {
	AccessToken string `yaml:"token"`
}

// fileStore is where a token handed to `auth github` lives between runs.
type fileStore struct{ file credstore.File }

func defaultStore() fileStore {
	return fileStore{file: credstore.Default()}
}

func (s fileStore) load() (string, error) {
	var t token
	err := s.file.Load(section, &t)
	return t.AccessToken, err
}

func (s fileStore) save(accessToken string) error {
	return s.file.Save(section, token{AccessToken: accessToken})
}
