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
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"

	"github.com/DnzzL/herdr-docket/internal/daemon"
	"github.com/DnzzL/herdr-docket/internal/grill"
	"github.com/DnzzL/herdr-docket/internal/text"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// phoneWidth is the narrowest terminal the questions prompt is read on: a
// phone in portrait running Termux.
const phoneWidth = 45

// questionsCmd is the Round screen on a terminal (ADR 0024), the line prompt
// for scripts and pipes.
func questionsCmd() error {
	_, src, err := settingsAndSource()
	if err != nil {
		return err
	}
	if terminal(os.Stdin) && terminal(os.Stdout) {
		m, ok, err := questionsPane(src)
		if err != nil {
			return err
		}
		if ok {
			return runPane(m)
		}
	}
	return answerQuestions(src, daemon.Liveness(time.Now()), os.Stdin, os.Stdout)
}

// terminal asks the tty driver, not the file mode: /dev/null is a character
// device too.
func terminal(f *os.File) bool { return term.IsTerminal(f.Fd()) }

// questionsPane is the pane `questions` opens: straight on the Round of the
// one Question card, on Home among the `?` rows when there are several. ok
// is false when there is none, for the line prompt to say so.
func questionsPane(src work.Source) (paneModel, bool, error) {
	cards, err := questionCards(src)
	if err != nil || len(cards) == 0 {
		return paneModel{}, false, err
	}
	m := newPane(src.List, func() (work.Source, error) { return src, nil })
	if len(cards) == 1 {
		m.open = cards[0].ID
	}
	return m, true, nil
}

// questionCards lists the Question cards, the owner's turn to answer: the
// Question flag decides, whatever the queue says about Open. A card a live run still
// holds is not the owner's turn yet, and `task answer` would refuse it.
func questionCards(src work.Source) ([]work.Task, error) {
	items, err := src.List()
	if err != nil {
		return nil, err
	}
	var cards []work.Task
	for _, it := range items {
		if it.Question && !runHeld(it.ID) {
			cards = append(cards, it)
		}
	}
	return cards, nil
}

// answerQuestions is `herdr-docket questions`: it lists the question cards,
// prints the chosen card's newest round context, walks its questions one at a
// time (Enter = the reco, a letter = that option, anything else = free text)
// and sends the assembled line through `task answer`. Every line fits
// phoneWidth.
func answerQuestions(src work.Source, live string, in io.Reader, out io.Writer) error {
	say(out, live)
	cards, err := questionCards(src)
	if err != nil {
		return err
	}
	if len(cards) == 0 {
		say(out, "no card in Question")
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
		r, _ := grill.LatestRound(full.Notes)
		rounds[i] = r.N
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
	round, ok := grill.LatestRound(card.Notes)
	if !ok || len(round.Questions) == 0 {
		return fmt.Errorf("%s: no ## Round note to answer", card.ID)
	}
	if round.Context != "" {
		say(out, "")
		for _, l := range strings.Split(round.Context, "\n") {
			say(out, l)
		}
	}
	var answers []grill.Answer
	for _, q := range round.Questions {
		a, err := askQuestion(r, out, q)
		if err != nil {
			return err
		}
		answers = append(answers, a)
	}
	line := grill.Line(answers)
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

// askQuestion shows one question and reads its answer: Enter the reco, a
// letter that option, anything else free text.
func askQuestion(r *bufio.Reader, out io.Writer, q grill.Question) (grill.Answer, error) {
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
			return grill.Answer{}, err
		}
		if line == "" && reco != "" {
			return grill.Answer{N: q.N, Letter: reco}, nil
		}
		for _, o := range q.Options {
			if strings.EqualFold(line, o.Letter) {
				return grill.Answer{N: q.N, Letter: o.Letter}, nil
			}
		}
		if line != "" {
			return grill.Answer{N: q.N, Text: line}, nil
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

// wrap breaks s into lines of at most width terminal cells, continuation
// lines indented like the first; a word longer than a line is cut.
func wrap(s string, width int) []string {
	indent := s[:len(s)-len(strings.TrimLeft(s, " "))]
	words := glue(strings.Fields(s))
	if len(words) == 0 {
		return []string{""}
	}
	room := max(width-len(indent), 1)
	var lines []string
	cur := indent
	for _, w := range words {
		for ansi.StringWidth(w) > room {
			if strings.TrimSpace(cur) != "" {
				lines = append(lines, cur)
			}
			head := ansi.Truncate(w, room, "")
			if head == "" { // a wide character on a one-cell line
				_, n := utf8.DecodeRuneInString(w)
				head = w[:n]
			}
			lines = append(lines, indent+head)
			w, cur = w[len(head):], indent
		}
		switch {
		case strings.TrimSpace(cur) == "":
			cur = indent + w
		case ansi.StringWidth(cur+" "+w) <= width:
			cur += " " + w
		default:
			lines = append(lines, cur)
			cur = indent + w
		}
	}
	return append(lines, cur)
}

// glue binds a lone : ? ! ; (French sets them apart with a space) to the
// word before with a no-break space, so no line starts with one.
func glue(words []string) []string {
	var out []string
	for _, w := range words {
		if len(out) > 0 && strings.Trim(w, ":?!;»") == "" {
			out[len(out)-1] += "\u00a0" + w
			continue
		}
		out = append(out, w)
	}
	return out
}

// fitting wraps what taskAnswer reports so its line fits phoneWidth too.
type fitting struct{ out io.Writer }

func (f *fitting) Write(p []byte) (int, error) {
	for _, l := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		say(f.out, l)
	}
	return len(p), nil
}
