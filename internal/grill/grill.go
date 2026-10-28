// Package grill reads the async grill's rounds off a card's notes. A round is
// a note the triager posts and a human reads without a tool:
//
//	## Round 2
//	Audio is sent as text today; upload.go builds the note.
//	1. What happens to the audio when it is sent?
//	   A. attached as a file (reco)
//	   B. text in a comment
//
// It is read back here for `task answer` and the questions view, so the
// format has one reader.
package grill

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Round is a card's newest round: its number, the context paragraph the
// triager writes between the heading and question 1 (lines trimmed, blank
// lines dropped, joined by newlines; empty when there is none), and its
// questions.
type Round struct {
	N         int
	Context   string
	Questions []Question
}

// Question is one question of a round: the line a run of lettered options
// hangs from, numbered or not. Before the first such question a numbered line
// with no options is context (the confirm round's numbered ASSUMPTIONS); after
// it, a numbered line with no options is a free-text question.
type Question struct {
	N       int
	Text    string
	Options []Option
}

// Option is one lettered answer; Reco marks the triager's recommendation.
type Option struct {
	Letter string
	Text   string
	Reco   bool
}

var (
	// A heading may carry the author prefix a GitHub thread puts on each
	// comment ("triager: ## Round 2").
	heading  = regexp.MustCompile(`^(?:\S+: )?## (\S+)(?: (\d+))?\s*$`)
	question = regexp.MustCompile(`^(\d+)\.\s+(.*\S)\s*$`)
	option   = regexp.MustCompile(`^\s*([A-Z])\.\s+(.*\S)\s*$`)
)

const recoMark = "(reco)"

// LatestRound finds the newest `## Round N` in a card's notes and parses its
// context and questions, which run until the next `## ` heading. ok is false
// when the notes hold no round.
func LatestRound(notes string) (r Round, ok bool) {
	lines := strings.Split(notes, "\n")
	start := -1
	for i, l := range lines {
		if m := heading.FindStringSubmatch(l); m != nil && m[1] == "Round" && m[2] != "" {
			start, r.N = i, atoi(m[2])
		}
	}
	if start < 0 {
		return Round{}, false
	}
	// A line becomes a question only when options follow it, so lines are
	// held as context until a run of options claims the one before it.
	var context []string
	open := false // the last non-blank line was an option of the last question
	for _, l := range lines[start+1:] {
		if heading.MatchString(l) {
			break
		}
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if m := option.FindStringSubmatch(l); m != nil && (open || len(context) > 0) {
			if !open {
				q := Question{N: len(r.Questions) + 1, Text: context[len(context)-1]}
				if n := question.FindStringSubmatch(q.Text); n != nil {
					q.N, q.Text = atoi(n[1]), n[2]
				}
				context = context[:len(context)-1]
				r.Questions = append(r.Questions, q)
				open = true
			}
			text, reco := strings.CutSuffix(m[2], recoMark)
			q := &r.Questions[len(r.Questions)-1]
			q.Options = append(q.Options, Option{Letter: m[1], Text: strings.TrimSpace(text), Reco: reco})
			continue
		}
		open = false
		// Once the questions have started, a numbered line is the next one,
		// options or not (a free-text question); before them it is context.
		if m := question.FindStringSubmatch(l); m != nil && len(r.Questions) > 0 {
			r.Questions = append(r.Questions, Question{N: atoi(m[1]), Text: m[2]})
			open = true
			continue
		}
		context = append(context, t)
	}
	r.Context = strings.Join(context, "\n")
	return r, true
}

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

// LatestAnswer is the number of the newest `## Answer N` in a card's notes,
// 0 when the owner has answered nothing yet.
func LatestAnswer(notes string) int {
	n := 0
	for _, l := range strings.Split(notes, "\n") {
		if m := heading.FindStringSubmatch(l); m != nil && m[1] == "Answer" && m[2] != "" {
			n = max(n, atoi(m[2]))
		}
	}
	return n
}

// Answer is one question's answer: option Letter, or free Text when Letter
// is empty.
type Answer struct {
	N            int
	Letter, Text string
}

// Line is a round's answers the way `task answer` posts them and the triager
// reads them: `1.A 2.B 3. free text`. Free text is one line, its whitespace
// collapsed.
func Line(answers []Answer) string {
	parts := make([]string, len(answers))
	for i, a := range answers {
		if a.Letter != "" {
			parts[i] = fmt.Sprintf("%d.%s", a.N, a.Letter)
		} else {
			parts[i] = fmt.Sprintf("%d. %s", a.N, strings.Join(strings.Fields(a.Text), " "))
		}
	}
	return strings.Join(parts, " ")
}
