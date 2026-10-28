package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DnzzL/herdr-docket/internal/daemon"
	"github.com/DnzzL/herdr-docket/internal/grill"
	"github.com/DnzzL/herdr-docket/internal/text"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// phoneWidth is the narrowest terminal the questions prompt is read on: a
// phone in portrait running Termux.
const phoneWidth = 45

func questionsCmd() error {
	_, src, err := settingsAndSource()
	if err != nil {
		return err
	}
	return answerQuestions(src, daemon.Liveness(time.Now()), os.Stdin, os.Stdout)
}

// questionCards lists the cards waiting on the owner's answer: the Question
// flag decides, whatever the queue says about Open.
func questionCards(src work.Source) ([]work.Task, error) {
	items, err := src.List()
	if err != nil {
		return nil, err
	}
	var cards []work.Task
	for _, it := range items {
		if it.Question {
			cards = append(cards, it)
		}
	}
	return cards, nil
}

// answerQuestions is `herdr-docket questions`: it lists the question cards,
// walks the chosen card's newest round one question at a time (Enter = the
// reco, a letter = that option, anything else = free text) and sends the
// assembled line through `task answer`. Every line fits phoneWidth.
func answerQuestions(src work.Source, live string, in io.Reader, out io.Writer) error {
	say(out, live)
	cards, err := questionCards(src)
	if err != nil {
		return err
	}
	if len(cards) == 0 {
		say(out, "no card waits on an answer")
		return nil
	}
	r := bufio.NewReader(in)
	// The list does not carry notes: each card is read for its round number.
	rounds := make([]int, len(cards))
	for i, c := range cards {
		full, err := src.Get(c.ID)
		if err != nil {
			return err
		}
		cards[i] = full
		rounds[i], _, _ = grill.LatestRound(full.Notes)
		head := fmt.Sprintf("%d. %s · round %d", i+1, c.ID, rounds[i])
		say(out, head)
		say(out, "   "+text.Truncate(c.Title, phoneWidth-3))
	}
	card := cards[0]
	if len(cards) > 1 {
		for {
			line, err := ask(r, out, "pick: ")
			if err != nil {
				return err
			}
			if i, err := strconv.Atoi(line); err == nil && i >= 1 && i <= len(cards) {
				card = cards[i-1]
				break
			}
		}
	}
	_, questions, ok := grill.LatestRound(card.Notes)
	if !ok || len(questions) == 0 {
		return fmt.Errorf("%s: no ## Round note to answer", card.ID)
	}
	var answers []string
	for _, q := range questions {
		a, err := askQuestion(r, out, q)
		if err != nil {
			return err
		}
		answers = append(answers, a)
	}
	line := strings.Join(answers, " ")
	say(out, line)
	reply, err := ask(r, out, "send? [Y/n] ")
	if err != nil {
		return err
	}
	if reply != "" && !strings.EqualFold(reply, "y") {
		say(out, "not sent")
		return nil
	}
	return taskAnswer(src, card.ID, line, &fitting{out})
}

// askQuestion shows one question and reads its answer as `N.A` or `N. text`.
func askQuestion(r *bufio.Reader, out io.Writer, q grill.Question) (string, error) {
	say(out, "")
	say(out, fmt.Sprintf("%d. %s", q.N, q.Text))
	reco := ""
	for _, o := range q.Options {
		l := fmt.Sprintf("   %s. %s", o.Letter, o.Text)
		if o.Reco {
			l += " (reco)"
			reco = o.Letter
		}
		say(out, l)
	}
	for {
		line, err := ask(r, out, "> ")
		if err != nil {
			return "", err
		}
		if line == "" && reco != "" {
			return fmt.Sprintf("%d.%s", q.N, reco), nil
		}
		for _, o := range q.Options {
			if strings.EqualFold(line, o.Letter) {
				return fmt.Sprintf("%d.%s", q.N, o.Letter), nil
			}
		}
		if line != "" {
			return fmt.Sprintf("%d. %s", q.N, line), nil
		}
	}
}

// ask prompts and reads one trimmed line. Input that ends before a line is
// an error: nothing is sent on a half-answered round.
func ask(r *bufio.Reader, out io.Writer, prompt string) (string, error) {
	fmt.Fprint(out, prompt)
	line, err := r.ReadString('\n')
	if err != nil && (line == "" || !errors.Is(err, io.EOF)) {
		fmt.Fprintln(out)
		return "", fmt.Errorf("input ended, nothing sent")
	}
	return strings.TrimSpace(line), nil
}

// say writes s word-wrapped to phoneWidth, continuation lines indented like
// the first.
func say(out io.Writer, s string) {
	for _, l := range wrap(s, phoneWidth) {
		fmt.Fprintln(out, l)
	}
}

func wrap(s string, width int) []string {
	indent := s[:len(s)-len(strings.TrimLeft(s, " "))]
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := indent
	for _, w := range words {
		for len([]rune(indent+w)) > width { // a word longer than a line is cut
			if strings.TrimSpace(cur) != "" {
				lines = append(lines, cur)
			}
			rw := []rune(w)
			lines = append(lines, indent+string(rw[:width-len(indent)]))
			w, cur = string(rw[width-len(indent):]), indent
		}
		switch {
		case strings.TrimSpace(cur) == "":
			cur = indent + w
		case len([]rune(cur+" "+w)) <= width:
			cur += " " + w
		default:
			lines = append(lines, cur)
			cur = indent + w
		}
	}
	return append(lines, cur)
}

// fitting wraps what taskAnswer reports so its line fits phoneWidth too.
type fitting struct{ out io.Writer }

func (f *fitting) Write(p []byte) (int, error) {
	for _, l := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		say(f.out, l)
	}
	return len(p), nil
}
