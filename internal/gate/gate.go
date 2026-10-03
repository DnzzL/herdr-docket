// Package gate decides whether the fleet may merge a pull request on its own.
// The decision is code, not prose in a persona: the agent that wrote the
// change never holds the gate, and neither does the one that judged it — the
// verifier records a verdict, the gate reads it (ADR 0013).
package gate

import (
	"fmt"
	"regexp"
	"strings"
)

// Verdict is the verifier's answer on a pull request.
const (
	Pass = "PASS"
	Fail = "FAIL"
)

// Checks is the pull request's CI, rolled up to the one word the gate needs.
type Checks string

const (
	ChecksPass    Checks = "pass"
	ChecksPending Checks = "pending"
	ChecksFail    Checks = "fail"
	// ChecksNone is a PR no check ran on. It holds the merge: a repo without
	// CI has nothing proving the change builds, and a verdict alone is one
	// agent's word.
	ChecksNone Checks = "none"
)

// PR is what the forge says about a pull request right now.
type PR struct {
	HeadSHA string
	PatchID string
	Checks  Checks
	Files   []string
	State   string // OPEN, CLOSED, MERGED
}

// Input is everything one decision reads.
type Input struct {
	PR PR
	// Verdict and VerdictPatch are the verifier's recorded answer and the
	// patch-id it was given on. A rebase that changes the diff changes the
	// patch-id, so a verdict on yesterday's diff does not cover today's.
	Verdict, VerdictPatch string
	Labels                []string
	// Owners are the target repo's CODEOWNERS patterns. Any changed file one
	// of them matches is a human's to merge.
	Owners []string
	// Auto is the queue's `merge: auto`. Without it a PASS still stops short.
	Auto bool
}

// Decide answers merge or hold, and for a hold, why — in words a human
// reading the task will act on.
func Decide(in Input) (merge bool, reason string) {
	switch {
	case in.PR.State != "OPEN":
		return false, fmt.Sprintf("the pull request is %s, not open", strings.ToLower(in.PR.State))
	case in.Verdict != Pass:
		if in.Verdict == "" {
			return false, "the verifier recorded no verdict"
		}
		return false, "the verifier's verdict is " + in.Verdict
	case in.VerdictPatch == "" || in.VerdictPatch != in.PR.PatchID:
		return false, "the diff changed after the verifier's verdict"
	case in.PR.Checks != ChecksPass:
		return false, "CI is " + string(in.PR.Checks)
	}
	for _, l := range in.Labels {
		if strings.EqualFold(l, "critical") {
			return false, "the task is labelled critical"
		}
	}
	for _, f := range in.PR.Files {
		for _, p := range in.Owners {
			if matches(p, f) {
				return false, fmt.Sprintf("%s is a CODEOWNERS path (%s)", f, p)
			}
		}
	}
	if !in.Auto {
		return false, "this queue does not merge on its own (merge: never)"
	}
	return true, ""
}

// ParseCodeowners returns the pattern of every rule in a CODEOWNERS file.
// The owners themselves do not matter here: any rule means a human reviews.
func ParseCodeowners(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, strings.Fields(line)[0])
	}
	return out
}

// matches applies one CODEOWNERS pattern to a repo-relative path, with
// GitHub's rules: a leading or middle slash anchors at the root, a trailing
// slash means a directory anywhere, a bare name matches at any depth, and a
// pattern naming a directory covers everything under it — except that a
// wildcard in the last segment stops at that level (`docs/*` is not
// `docs/a/b.md`).
func matches(pattern, file string) bool {
	dir := strings.HasSuffix(pattern, "/")
	p := strings.TrimSuffix(pattern, "/")
	anchored := strings.HasPrefix(p, "/") || strings.Contains(strings.TrimPrefix(p, "/"), "/")
	p = strings.TrimPrefix(p, "/")

	var re strings.Builder
	if anchored {
		re.WriteString("^")
	} else {
		re.WriteString("(^|.*/)")
	}
	for i := 0; i < len(p); i++ {
		switch {
		case strings.HasPrefix(p[i:], "**/"):
			re.WriteString("(.*/)?")
			i += 2
		case strings.HasPrefix(p[i:], "**"):
			re.WriteString(".*")
			i++
		case p[i] == '*':
			re.WriteString("[^/]*")
		case p[i] == '?':
			re.WriteString("[^/]")
		default:
			re.WriteString(regexp.QuoteMeta(string(p[i])))
		}
	}
	last := p[strings.LastIndex(p, "/")+1:]
	switch {
	case dir:
		re.WriteString("/.*$")
	case strings.ContainsAny(last, "*?"):
		re.WriteString("$")
	default:
		re.WriteString("(/.*)?$")
	}
	ok, _ := regexp.MatchString(re.String(), file)
	return ok
}
