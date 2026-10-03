package gate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Forge is the pull-request host, reduced to what the gate and the verdict
// command ask of it.
type Forge interface {
	PR(url string) (PR, error)
	// Merge squashes the PR, refusing if its head moved past headSHA — the
	// commit the gate decided on.
	Merge(url, headSHA string) error
	Comment(url, body string) error
}

// GH is the Forge behind the gh CLI, with whatever account it is signed in as.
type GH struct{}

func (GH) PR(url string) (PR, error) {
	var view struct {
		HeadRefOid string `json:"headRefOid"`
		State      string `json:"state"`
		Files      []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	out, err := gh("pr", "view", url, "--json", "headRefOid,state,files")
	if err != nil {
		return PR{}, err
	}
	if err := json.Unmarshal(out, &view); err != nil {
		return PR{}, fmt.Errorf("gh pr view %s: %w", url, err)
	}
	pr := PR{HeadSHA: view.HeadRefOid, State: view.State}
	for _, f := range view.Files {
		pr.Files = append(pr.Files, f.Path)
	}
	if pr.PatchID, err = patchID(url); err != nil {
		return PR{}, err
	}
	pr.Checks = checks(url)
	return pr, nil
}

func (GH) Merge(url, headSHA string) error {
	_, err := gh("pr", "merge", url, "--squash", "--delete-branch", "--match-head-commit", headSHA)
	return err
}

func (GH) Comment(url, body string) error {
	_, err := gh("pr", "comment", url, "--body", body)
	return err
}

// patchID is `git patch-id --stable` over the PR's diff: the same change
// rebased onto a newer base keeps its id, a change to the change does not.
func patchID(url string) (string, error) {
	diff, err := gh("pr", "diff", url)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("git", "patch-id", "--stable")
	cmd.Stdin = bytes.NewReader(diff)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git patch-id: %w", err)
	}
	f := strings.Fields(string(out))
	if len(f) == 0 {
		return "", fmt.Errorf("pull request %s has an empty diff", url)
	}
	return f[0], nil
}

// checks rolls the PR's checks up to one word. gh exits non-zero while a
// check fails or is pending and still prints the JSON, so the output is read
// whatever the exit code; no output at all is a PR no check ran on.
func checks(url string) Checks {
	out, _ := exec.Command("gh", "pr", "checks", url, "--json", "bucket").Output()
	var rows []struct {
		Bucket string `json:"bucket"`
	}
	if json.Unmarshal(out, &rows) != nil || len(rows) == 0 {
		return ChecksNone
	}
	pending := false
	for _, r := range rows {
		switch r.Bucket {
		case "fail", "cancel":
			return ChecksFail
		case "pending":
			pending = true
		}
	}
	if pending {
		return ChecksPending
	}
	return ChecksPass
}

func gh(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("gh", args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %v: %s", args[0]+" "+args[1], err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
