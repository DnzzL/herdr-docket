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
	var context []string
	for _, l := range lines[start+1:] {
		if heading.MatchString(l) {
			break
		}
		if m := question.FindStringSubmatch(l); m != nil {
			r.Questions = append(r.Questions, Question{N: atoi(m[1]), Text: m[2]})
			continue
		}
		if len(r.Questions) == 0 {
			if t := strings.TrimSpace(l); t != "" {
				context = append(context, t)
			}
			continue
		}
		if m := option.FindStringSubmatch(l); m != nil {
			text, reco := strings.CutSuffix(m[2], recoMark)
			q := &r.Questions[len(r.Questions)-1]
			q.Options = append(q.Options, Option{Letter: m[1], Text: strings.TrimSpace(text), Reco: reco})
		}
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
