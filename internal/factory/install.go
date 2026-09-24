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

// The fleet-side personas: the three stages whose workdir is the fleet itself
// (intake, stall and lookback read the queue, not a project). The per-project
// personas — dev, reviewer — are copied by hand from docs/examples.md, since
// only the owner knows the repo they work in.
//
//go:embed personas/intake.md
var intakePersona string

//go:embed personas/stall.md
var stallPersona string

//go:embed personas/lookback.md
var lookbackPersona string

var personas = []struct{ name, body string }{
	{"intake", intakePersona},
	{"stall", stallPersona},
	{"lookback", lookbackPersona},
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
        -d "Poll the sources in your persona's prompt — Sentry via mcp_config,
        gh issue list — and apply the gate. Report only."

  - name: review-sweep
    cron: "30 9 * * 1-5"
    repo: ~/fleet
    workspace: root
    model: sonnet
    prompt: |
      herdr-docket task create "Sweep: review the oldest un-reviewed PR" -a reviewer \
        -d "Review the oldest open pull request without a verdict, then re-task
        yourself for the rest. Your merge policy is in your persona."

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
        assigned to pm, evidence on each."
`

// Install writes the loop into dir: the fleet-side personas, then the
// schedules that drive them when the automations plugin answers for its
// config. Everything it writes is reported; everything it leaves alone is
// left alone without comment.
func Install(dir string, out io.Writer) error {
	if err := os.MkdirAll(filepath.Join(dir, "agents"), 0o755); err != nil {
		return err
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
		body := strings.Replace(p.body, "workdir: ~/fleet", "workdir: "+dir, 1)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "factory: wrote agents/%s/AGENT.md\n", p.name)
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
	if strings.Contains(string(raw), "- name: intake") {
		fmt.Fprintf(out, "factory: %s already drives the loop\n", path)
		return afterInstall(out)
	}
	// Insert after the automations key, so the entries land in the list and
	// nothing already in the file moves. A file with no such key gets one
	// prepended: a second key would not parse, and a rewrite of the owner's
	// file is not this command's to make.
	key := regexp.MustCompile(`(?m)^automations:[ \t]*$`)
	if loc := key.FindIndex(raw); loc != nil {
		raw = append(raw[:loc[1]+1], append([]byte(block), raw[loc[1]+1:]...)...)
	} else {
		raw = append([]byte("automations:\n"+block+"\n"), raw...)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "factory: appended the loop to %s\n", path)
	return afterInstall(out)
}

// afterInstall is the one line of orientation every path ends with: the dial
// to turn before the schedule is trusted, and where the rest is written down.
func afterInstall(out io.Writer) error {
	fmt.Fprintln(out, `factory: next — read docs/factory.md; the intake entry files nothing until you delete "Report only."`)
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
