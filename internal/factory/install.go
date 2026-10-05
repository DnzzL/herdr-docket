// Package factory installs the factory loop into an existing fleet: the
// fleet-side personas the loop runs on, and the schedules that drive them.
// It writes the same files a human would copy from docs/examples.md and
// docs/factory.md — never overwriting what the fleet already has, and never
// touching the fleet's own config, because the loop runs over queues this
// package was never asked about.
package factory

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/DnzzL/herdr-docket/internal/hostpath"
)

// The loop's five personas, in two kinds. The fleet-side three — intake, stall
// and lookback — read the queue, so their workdir is the fleet itself and they
// run the moment they land. The project-side two — dev and reviewer —
// work a repo only the owner can name, so they land paused, carrying the
// example's placeholder workdir: every stage the loop refers to exists from
// the first install, and none of them runs until a human has pointed it
// somewhere real.
//
//go:embed personas/intake.md
var intakePersona string

//go:embed personas/stall.md
var stallPersona string

//go:embed personas/lookback.md
var lookbackPersona string

//go:embed personas/dev.md
var devPersona string

//go:embed personas/reviewer.md
var reviewerPersona string

// The project-side two share their method through roles (ADR 0005): the
// persona says which repo, the role says how to work. A second project's dev
// is then one short file naming the same role.
//
//go:embed roles/dev.md
var devRole string

//go:embed roles/reviewer.md
var reviewerRole string

var roles = []struct{ name, body string }{
	{"dev", devRole},
	{"reviewer", reviewerRole},
}

// placeholder is the workdir the project-side personas carry out of
// docs/examples.md — the line the owner edits before resuming one.
const placeholder = "workdir: ~/Projects/myapp\n"

var personas = []struct {
	name, body string
	project    bool
}{
	{"intake", intakePersona, false},
	{"stall", stallPersona, false},
	{"lookback", lookbackPersona, false},
	{"dev", devPersona, true},
	{"reviewer", reviewerPersona, true},
}

// entries are the four automations that drive the loop, verbatim as
// docs/factory.md shows them (a drift test pins the two together). The only
// thing Install does to them is point `repo:` at the fleet being installed.
const entries = `  # Delete "Report only." from the intake entry after you have read one
  # report and liked the gate. Until then nothing is filed.
  - name: intake
    cron: "15 */4 * * *"
    repo: ~/fleet
    workspace: root
    model: haiku
    prompt: |
      herdr-docket task create "Intake: turn new feedback into fleet tasks" -a intake \
        -d "Poll the sources your persona and mcp_config name and apply the
        gate. Report only."

  - name: stall-sweep
    cron: "0 13 * * *"
    repo: ~/fleet
    workspace: root
    model: haiku
    prompt: |
      herdr-docket task create "Sweep: nudge work that ended short" -a stall \
        -d "Read the queue and history.jsonl for the three stalls your persona
        names. Nudge with notes; decide nothing."

  - name: lookback
    cron: "0 10 * * 1"
    repo: ~/fleet
    workspace: root
    model: opus
    prompt: |
      herdr-docket task create "Lookback: what keeps coming back?" -a lookback \
        -d "Last 30 days against the 30 before. One follow-up per pattern,
        assigned to dev, evidence on each."

  - name: merge-digest
    cron: "30 7 * * *"
    repo: ~/fleet
    workspace: root
    model: haiku
    prompt: |
      Daily merge digest — one issue per repo listing every open pull request
      labelled merge-needed. It decides nothing; it only lists: exactly one
      open issue titled "Merges waiting" per repo, rewritten each run, closed
      when the repo has nothing waiting.

      Queue repos: for every "dir:" under "sources:" in the fleet.yaml inside
      the dir printed by herdr plugin config-dir dnzzl.herdr-docket (a source
      with no "dir:" is the fleet's own queue at the top-level "dir:"), take
      git -C <dir> remote get-url origin and strip any
      git@github.com:/https://github.com/ prefix and .git suffix. An
      owner/repo answer is a repo to read — each repo once, even when two
      queues share it. No remote, or not github.com: that queue has no pull
      requests — say so and skip it. A kind: github source names its repo in
      the "repo:" of its "github:" block instead.

      Per repo: gh pr list --repo <owner/repo> --state open --label
      merge-needed --limit 1000 --json number,url. An empty list is the
      normal answer; a gh error is not — report it and move on. No waiting
      PRs: close that repo's open "Merges waiting" issue if there is one
      (gh issue close <n> --comment "nothing waiting — no pull request is
      labelled merge-needed"), then say so and move to the next repo.

      Per waiting PR, four facts — read them, never guess them. Link: from
      the list. Task id: grep -rl the url under <dir>/backlog/tasks/ and take
      the frontmatter id: of the file that also carries "is held for a
      human", under the source name fleet.yaml gives that dir; if no file
      mentions the url, the last "task" beside it in
      ~/.local/state/herdr/plugins/dnzzl.herdr-docket/history.jsonl; if
      neither, "not recorded". Reason: the text after the dash on that
      "is held for a human —" line; no such line, "not recorded". Waiting
      since: the last created_at of a merge-needed label event in gh api
      repos/<owner/repo>/issues/<n>/timeline --paginate --jq
      '.[]|select(.event=="labeled" and .label.name=="merge-needed")|.created_at'
      (piped through tail -1); no event, "unknown".

      A repo with waiting PRs keeps exactly one open "Merges waiting" issue:
      gh issue create --repo <owner/repo> --title "Merges waiting" --body-file -
      when none is open, gh issue edit <n> --repo <owner/repo> --body-file -
      on the one already open, and a second open one is closed as superseded
      (gh issue close <m> --comment "superseded — one digest issue per repo").
      The body: one bullet per PR — link, task id, reason, waiting since —
      under a line saying it is rewritten daily and lists only.

      End with one line per repo: what you listed, what you closed.
`

// entryName matches a shipped schedule's name line; the capture is the name
// Install checks the owner's config for, one entry at a time.
var entryName = regexp.MustCompile(`(?m)^  - name: (\S+)[ \t]*$`)

// shippedEntry is one schedule as the block ships it: its name, and the chunk
// that would be appended on its own — the comments that lead the entry (if
// any), the entry, and the blank line down to the next one.
type shippedEntry struct{ name, body string }

// shipped splits block into one chunk per schedule, in ship order.
func shipped(block string) []shippedEntry {
	locs := entryName.FindAllStringSubmatchIndex(block, -1)
	out := make([]shippedEntry, 0, len(locs))
	for i, loc := range locs {
		start := 0
		if i > 0 {
			start = loc[0]
		}
		end := len(block)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out = append(out, shippedEntry{block[loc[2]:loc[3]], block[start:end]})
	}
	return out
}

// hasEntry reports whether the config already names the schedule: a line that
// is exactly `- name: X`, so `- name: intake-followup` does not stand in for
// the intake entry and the indentation the owner chose does not matter.
func hasEntry(raw []byte, name string) bool {
	want := "- name: " + name
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

// Install writes the loop into dir: the fleet-side personas, then the
// schedules that drive them when the automations plugin answers for its
// config. Everything it writes is reported; everything it leaves alone is
// left alone without comment.
func Install(dir string, out io.Writer) error {
	if err := os.MkdirAll(filepath.Join(dir, "agents"), 0o755); err != nil {
		return err
	}
	// Roles before the agents that name them: an agent whose role is missing
	// is grounded. A role the fleet already has is its owner's and is kept.
	if err := os.MkdirAll(filepath.Join(dir, "roles"), 0o755); err != nil {
		return err
	}
	for _, r := range roles {
		path := filepath.Join(dir, "roles", r.name+".md")
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(out, "factory: skipped roles/%s.md — it already exists\n", r.name)
			continue
		}
		if err := os.WriteFile(path, []byte(r.body), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "factory: wrote roles/%s.md\n", r.name)
	}
	for _, p := range personas {
		path := filepath.Join(dir, "agents", p.name, "AGENT.md")
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(out, "factory: skipped agents/%s — it already exists\n", p.name)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		body, note := p.body, ""
		if p.project {
			// Paused on arrival, and the workdir left as the example wrote it:
			// a persona pointed at a repo that isn't yours is a guess, and a
			// guess that runs is worse than one that waits.
			body = strings.Replace(body, placeholder, placeholder+"disabled: true\n", 1)
			note = fmt.Sprintf(" — paused; point its workdir at your repo, then: herdr-docket agent resume %s", p.name)
		} else {
			body = strings.Replace(body, "workdir: ~/fleet", "workdir: "+dir, 1)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "factory: wrote agents/%s/AGENT.md%s\n", p.name, note)
	}

	cfg, err := automationsConfigDir()
	if err != nil {
		fmt.Fprintln(out, "factory: automations config not found — the personas await schedules (see docs/factory.md)")
		return afterInstall(out)
	}
	path := filepath.Join(cfg, "automations.yaml")
	block := strings.ReplaceAll(entries, "repo: ~/fleet", "repo: "+dir)
	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if err := os.WriteFile(path, []byte("automations:\n"+block), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "factory: created %s with the loop's four entries\n", path)
		return afterInstall(out)
	case err != nil:
		return err
	}
	// The entries the config does not name yet, in ship order: a fleet that
	// installed before an entry existed receives it from a re-run, and one
	// that already has them all is left alone.
	var add strings.Builder
	var added []string
	for _, e := range shipped(block) {
		if hasEntry(raw, e.name) {
			continue
		}
		add.WriteString(e.body)
		added = append(added, e.name)
	}
	if add.Len() == 0 {
		fmt.Fprintf(out, "factory: %s already drives the loop\n", path)
		return afterInstall(out)
	}
	// Insert after the automations key, so the entries land in the list and
	// nothing already in the file moves. A file with no such key gets one
	// prepended: a second key would not parse, and a rewrite of the owner's
	// file is not this command's to make.
	key := regexp.MustCompile(`(?m)^automations:[ \t]*$`)
	if loc := key.FindIndex(raw); loc != nil {
		raw = append(raw[:loc[1]+1], append([]byte(add.String()), raw[loc[1]+1:]...)...)
	} else {
		raw = append([]byte("automations:\n"+add.String()+"\n"), raw...)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "factory: appended %s to %s\n", strings.Join(added, ", "), path)
	return afterInstall(out)
}

// afterInstall is the orientation every path ends with: the two dials to turn
// before the loop is trusted, and where the rest is written down.
func afterInstall(out io.Writer) error {
	fmt.Fprintln(out, `factory: next — read docs/factory.md; the intake entry files nothing until you delete "Report only."`)
	fmt.Fprintln(out, "factory: dev and reviewer are paused until their workdir names your repo; then name the reviewer as the queue's verifier in fleet.yaml")
	return nil
}

// automationsConfigDir asks herdr where the automations plugin keeps its
// config. An error means the plugin is not there, which is an answer and not
// a failure: the personas still land and the note says what is still owed.
func automationsConfigDir() (string, error) {
	out, err := exec.Command(hostpath.Bin(), "plugin", "config-dir", "dnzzl.automations").Output()
	if err != nil {
		return "", err
	}
	d := strings.TrimSpace(string(out))
	if d == "" {
		return "", fmt.Errorf("herdr named no config dir for dnzzl.automations")
	}
	return d, nil
}
