package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// An ADR number names one decision: two files carrying 0020 in parallel
// pull requests leave a reader following "ADR-0020" unable to tell which
// record is meant. Read the directory as it is on disk — the next collision
// must go red without anyone updating a list.
var adrNumber = regexp.MustCompile(`^(\d{4})-.*\.md$`)

func TestEveryADRHasItsOwnNumber(t *testing.T) {
	entries, err := os.ReadDir("docs/adr")
	if err != nil {
		t.Fatal(err)
	}
	byNumber := map[string][]string{}
	for _, entry := range entries {
		match := adrNumber.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		byNumber[match[1]] = append(byNumber[match[1]], entry.Name())
	}
	for number, files := range byNumber {
		if len(files) < 2 {
			continue
		}
		sort.Strings(files)
		t.Errorf("ADR %s is used by %d files: %s", number, len(files), strings.Join(files, ", "))
	}
	if len(byNumber) == 0 {
		t.Fatalf("found no ADR files in %s", filepath.Join("docs", "adr"))
	}
}
