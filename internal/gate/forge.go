package gate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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
	// Codeowners reads the repo's CODEOWNERS patterns from the ref a PR
	// targets, not from any checkout on disk: a workdir behind the base is
	// the stale file, a workdir on a branch without the file is nothing
	// (TASK-57). nil is no file on that ref.
	Codeowners(prURL string) ([]string, error)
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

// codeownerPaths are where GitHub looks for CODEOWNERS, in its own order.
var codeownerPaths = []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"}

// Codeowners fetches the CODEOWNERS of the PR's base ref through the gh API
// (repos/{owner}/{repo}/contents/{path}?ref={base}), first of the three
// places GitHub looks. No file on the base is nil, no error — unchanged from
// the disk read.
func (GH) Codeowners(prURL string) ([]string, error) {
	owner, repo, err := githubRepo(prURL)
	if err != nil {
		return nil, err
	}
	ref, err := baseRef(prURL)
	if err != nil {
		return nil, fmt.Errorf("the base ref: %w", err)
	}
	for _, path := range codeownerPaths {
		raw, err := contents(owner, repo, ref, path)
		switch {
		case err == nil:
			return ParseCodeowners(string(raw)), nil
		case errors.Is(err, fs.ErrNotExist):
			continue // GitHub looks in the next place.
		default:
			return nil, err
		}
	}
	return nil, nil
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

// contents is `gh api` for one file's raw bytes at a ref, the repo named
// explicitly — the daemon's cwd is no git checkout, so `gh api
// repos/{owner}/{repo}` placeholder expansion cannot be relied on, and the
// ref rides in the url: `gh api -f` form values silently turn the GET into
// a POST, which the contents endpoint answers 404. It asks for
// `application/vnd.github.raw` so gh returns the file as stored, no base64
// JSON to decode. A path not on the ref comes back as fs.ErrNotExist — the
// caller's keep-looking — and anything else surfaces as its own error.
func contents(owner, repo, ref, path string) ([]byte, error) {
	var stderr bytes.Buffer
	end := fmt.Sprintf("repos/%s/%s/contents/%s?ref=%s", owner, repo, path, url.QueryEscape(ref))
	cmd := exec.Command("gh", "api", end, "-H", "Accept: application/vnd.github.raw")
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		// gh api reports a missing path as `gh: Not Found (HTTP 404)`; no
		// sentinel is returned, so the message is the only tell. Only gh's
		// own stderr carries that tell — the endpoint it rode in on names
		// the owner, repo and ref, and any of them could contain "404".
		// Matching more would turn credentials/network trouble into "no
		// file" and merge a PR unguarded.
		if strings.Contains(stderr.String(), "HTTP 404") {
			return nil, fmt.Errorf("repos/%s/%s/contents/%s: %w", owner, repo, path, fs.ErrNotExist)
		}
		return nil, fmt.Errorf("gh api %s: %v: %s", end, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// githubRepo reads owner and repo out of the PR url the pipeline carries:
// the whole https://github.com/owner/repo/pull/N form, or nothing.
func githubRepo(prURL string) (owner, repo string, err error) {
	const marker = "github.com/"
	i := strings.Index(prURL, marker)
	if i < 0 {
		return "", "", fmt.Errorf("not a GitHub pull request url: %s", prURL)
	}
	parts := strings.SplitN(prURL[i+len(marker):], "/", 4) // owner/repo/pull/N
	if len(parts) < 4 || parts[2] != "pull" {
		return "", "", fmt.Errorf("not a GitHub pull request url: %s", prURL)
	}
	return parts[0], parts[1], nil
}

// baseRef is the branch the PR merges into: the ref whose CODEOWNERS hold.
func baseRef(prURL string) (string, error) {
	out, err := gh("pr", "view", prURL, "--json", "baseRefName")
	if err != nil {
		return "", err
	}
	var v struct {
		BaseRefName string `json:"baseRefName"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return "", fmt.Errorf("base ref of %s: %w", prURL, err)
	}
	return v.BaseRefName, nil
}
