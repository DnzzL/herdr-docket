package gate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
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
	// AddLabel applies label to the PR, creating it in the repo first if the
	// repo does not have it yet.
	AddLabel(url, label string) error
	// RemoveLabel takes label off the PR. A PR that does not carry it is
	// already the state the caller wanted, not an error.
	RemoveLabel(url, label string) error
}

// RepoRef splits a pull-request URL into the repository that hosts it
// ("owner/repo") and the number the PR is known by ("7") — the two halves
// every caller needs to name one: the gh CLI wants the repo, and
// owner/repo#7 is how a human reads it.
func RepoRef(prURL string) (repo, number string, err error) {
	u, err := url.Parse(prURL)
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "pull" || parts[0] == "" || parts[1] == "" || parts[3] == "" {
		return "", "", fmt.Errorf("%q does not name a pull request", prURL)
	}
	return parts[0] + "/" + parts[1], parts[3], nil
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

func (GH) AddLabel(url, label string) error {
	repo, _, err := RepoRef(url)
	if err != nil {
		return err
	}
	// Creating is best effort: the label existing already makes this fail,
	// which is the outcome wanted, and a label the repo really lacks is
	// caught below, where `pr edit` refuses to apply what does not exist.
	_, _ = gh("label", "create", label, "--repo", repo)
	_, err = gh("pr", "edit", url, "--add-label", label)
	return err
}

// RemoveLabel reads the PR's labels first rather than removing blind: a repo
// that never needed the label does not have it, and gh refuses to remove a
// label it has never heard of — a refusal that would log on every clean merge
// for a state that is already the one wanted.
func (GH) RemoveLabel(url, label string) error {
	var view struct {
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	out, err := gh("pr", "view", url, "--json", "labels")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(out, &view); err != nil {
		return fmt.Errorf("gh pr view %s: %w", url, err)
	}
	for _, l := range view.Labels {
		if strings.EqualFold(l.Name, label) {
			_, err = gh("pr", "edit", url, "--remove-label", label)
			return err
		}
	}
	return nil
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
