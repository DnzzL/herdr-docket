// Package grill reads the async grill's rounds off a card's notes. A round is
// a note the triager posts and a human reads without a tool:
//
//	## Round 2
//	1. What happens to the audio when it is sent?
//	   A. attached as a file (reco)
//	   B. text in a comment
//
// It is read back here for `task answer` and the questions view, so the
// format has one reader.
package grill

import (
	"regexp"
	"strconv"
	"strings"
)

// Question is one numbered question of a round, with its lettered options.
// A question with no options takes a free-text answer.
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
	option   = regexp.MustCompile(`^\s+([A-Z])\.\s+(.*\S)\s*$`)
)

const recoMark = "(reco)"

// LatestRound finds the newest `## Round N` in a card's notes and parses its
// questions, which run until the next `## ` heading. ok is false when the
// notes hold no round.
func LatestRound(notes string) (n int, questions []Question, ok bool) {
	lines := strings.Split(notes, "\n")
	start := -1
	for i, l := range lines {
		if m := heading.FindStringSubmatch(l); m != nil && m[1] == "Round" && m[2] != "" {
			start, n = i, atoi(m[2])
		}
	}
	if start < 0 {
		return 0, nil, false
	}
	for _, l := range lines[start+1:] {
		if heading.MatchString(l) {
			break
		}
		if m := question.FindStringSubmatch(l); m != nil {
			questions = append(questions, Question{N: atoi(m[1]), Text: m[2]})
			continue
		}
		if m := option.FindStringSubmatch(l); m != nil && len(questions) > 0 {
			text, reco := strings.CutSuffix(m[2], recoMark)
			q := &questions[len(questions)-1]
			q.Options = append(q.Options, Option{Letter: m[1], Text: strings.TrimSpace(text), Reco: reco})
		}
	}
	return n, questions, true
}

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
