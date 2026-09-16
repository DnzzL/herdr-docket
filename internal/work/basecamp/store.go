package basecamp

import (
	// Aliased: this package already has a `credentials` — the in-memory
	// holder in auth.go — and that is the name the adapter speaks.
	credstore "github.com/DnzzL/herdr-docket/internal/credentials"
)

// section is this adapter's block of credentials.yaml. The file itself —
// its mode, its other sections, the read-modify-write that keeps them — is
// internal/credentials' business, because several backends write to it.
const section = "basecamp"

// fileStore is where a Basecamp token lives between runs.
type fileStore struct{ file credstore.File }

func defaultStore() fileStore {
	return fileStore{file: credstore.Default()}
}

func (s fileStore) load() (Token, error) {
	var t Token
	err := s.file.Load(section, &t)
	return t, err
}

func (s fileStore) save(t Token) error {
	return s.file.Save(section, t)
}
