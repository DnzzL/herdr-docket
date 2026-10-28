package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/DnzzL/herdr-docket/internal/grill"
)

// inlineWidth is the narrowest screen a question's buttons may share a line
// on; below it each button gets its own line, so a thumb hits one at a time.
const inlineWidth = 60

// roundDoneMsg leaves the Round for Home, with the line Home shows: the
// confirmation of a sent answer, or nothing on Esc. answered is the card a
// sent answer moved out of Your turn.
type roundDoneMsg struct{ answered, status string }

// sentMsg is what Send's `task answer` said, once it returned.
type sentMsg struct {
	out string
	err error
}

// control is a kind of button on the Round.
type control int

const (
	optionButton control = iota // one of a question's lettered options
	wordsButton                 // a question's "or in plain words…"
	sendButton                  // Send, under the questions
)

// focus names one button: question q's option opt, q's plain-words button,
// or Send (q and opt zero for the last two that do not use them).
type focus struct {
	kind   control
	q, opt int
}

// roundModel is the Round screen: the round's context, each question's
// options as buttons with the reco preselected, a plain-words field per
// question, and Send. It writes nothing itself: send is the `task answer`
// path, lock and status checks included.
// Keys: ↑/↓ j/k question, ←/→ h/l option, a letter picks that option, Enter
// picks or sends, Tab toggles free text, Esc back, q quits.
type roundModel struct {
	id, title     string
	round         grill.Round
	send          func(line string) (string, error)
	answers       []grill.Answer
	at            focus
	editing       bool // the focused question's text field takes the keys
	sending       bool // Send's `task answer` has not returned yet
	width, height int
	top           int
	status        string

	// lines and hits are the screen as last laid out: what View draws and
	// where a tap lands, from the same pass.
	lines []string
	hits  []roundHit
}

func newRound(id, title string, round grill.Round, send func(string) (string, error), width, height int) roundModel {
	m := roundModel{id: id, title: title, round: round, send: send, width: width, height: height}
	m.answers = make([]grill.Answer, len(round.Questions))
	for i, q := range round.Questions {
		m.answers[i].N = q.N
		for _, o := range q.Options {
			if o.Reco {
				m.answers[i].Letter = o.Letter
			}
		}
	}
	m.at = m.question(0)
	return m.laidOut(false)
}

// question is the button focus lands on in question q: its answer's option,
// its plain-words button when answered in words or optionless, else its
// first option. Past the last question is Send.
func (m roundModel) question(q int) focus {
	if q >= len(m.round.Questions) {
		return focus{kind: sendButton}
	}
	opts := m.round.Questions[q].Options
	for i, o := range opts {
		if o.Letter == m.answers[q].Letter {
			return focus{kind: optionButton, q: q, opt: i}
		}
	}
	if m.answers[q].Text == "" && len(opts) > 0 {
		return focus{kind: optionButton, q: q}
	}
	return focus{kind: wordsButton, q: q}
}

func (m roundModel) answered(q int) bool {
	return m.answers[q].Letter != "" || strings.TrimSpace(m.answers[q].Text) != ""
}

func (m roundModel) complete() bool {
	for q := range m.answers {
		if !m.answered(q) {
			return false
		}
	}
	return true
}

// Update handles one message and lays the screen out once, after it.
func (m roundModel) Update(msg tea.Msg) (roundModel, tea.Cmd) {
	var cmd tea.Cmd
	follow := false
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case sentMsg:
		m.sending = false
		if msg.err != nil {
			m.status = msg.err.Error()
			break
		}
		done := roundDoneMsg{answered: m.id, status: strings.TrimSpace(msg.out)}
		cmd = func() tea.Msg { return done }
	case tea.MouseMsg:
		m, cmd = m.mouse(msg)
	case tea.KeyMsg:
		m, cmd = m.key(msg)
		follow = true
	default:
		return m, nil
	}
	return m.laidOut(follow), cmd
}

func (m roundModel) mouse(msg tea.MouseMsg) (roundModel, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelDown:
		m.top += wheelStep
	case tea.MouseButtonWheelUp:
		m.top -= wheelStep
	case tea.MouseButtonLeft:
		for _, h := range m.hits {
			if msg.Y >= h.y0 && msg.Y <= h.y1 && msg.X >= h.x0 && msg.X < h.x1 {
				m.at = h.at
				return m.press()
			}
		}
	}
	return m, nil
}

// press is Enter, or a tap, on the focused button.
func (m roundModel) press() (roundModel, tea.Cmd) {
	switch m.at.kind {
	case sendButton:
		return m.submit()
	case optionButton:
		m.answers[m.at.q].Letter = m.round.Questions[m.at.q].Options[m.at.opt].Letter
		m.editing = false
	case wordsButton:
		m.answers[m.at.q].Letter = ""
		m.editing = !m.editing
	}
	return m, nil
}

// submit sends the answers as a command: `task answer` reads and writes the
// queue, which may be a GitHub board, and the screen must not freeze on it.
func (m roundModel) submit() (roundModel, tea.Cmd) {
	if m.sending {
		return m, nil
	}
	if !m.complete() {
		m.status = "answer every question first"
		return m, nil
	}
	m.sending, m.status = true, "sending…"
	send, line := m.send, grill.Line(m.answers)
	return m, func() tea.Msg {
		out, err := send(line)
		return sentMsg{out: out, err: err}
	}
}

func (m roundModel) key(msg tea.KeyMsg) (roundModel, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	if m.editing {
		a := &m.answers[m.at.q]
		switch msg.Type {
		case tea.KeyEnter, tea.KeyTab, tea.KeyEsc:
			m.editing = false
		case tea.KeyBackspace:
			if r := []rune(a.Text); len(r) > 0 {
				a.Text = string(r[:len(r)-1])
			}
		case tea.KeySpace:
			a.Text += " "
		case tea.KeyRunes:
			a.Text += strings.NewReplacer("\n", " ", "\r", " ").Replace(string(msg.Runes))
		}
		return m, nil
	}
	switch k {
	case "q":
		return m, tea.Quit
	case "esc":
		return m, func() tea.Msg { return roundDoneMsg{} }
	case "up", "k":
		m.at = m.question(max(m.row()-1, 0))
	case "down", "j":
		m.at = m.question(min(m.row()+1, len(m.round.Questions)))
	case "left", "h":
		m.at = m.option(-1)
	case "right", "l":
		m.at = m.option(+1)
	case "enter":
		return m.press()
	case "tab":
		if m.at.kind != sendButton {
			m.at = focus{kind: wordsButton, q: m.at.q}
			return m.press()
		}
	default:
		if m.at.kind != sendButton && len(k) == 1 {
			for i, o := range m.round.Questions[m.at.q].Options {
				if strings.EqualFold(k, o.Letter) {
					m.at = focus{kind: optionButton, q: m.at.q, opt: i}
					return m.press()
				}
			}
		}
	}
	return m, nil
}

// row is the focused row of the screen's column of questions: the question,
// or len(questions) for Send.
func (m roundModel) row() int {
	if m.at.kind == sendButton {
		return len(m.round.Questions)
	}
	return m.at.q
}

// option moves focus d buttons along its question's row, plain words last.
func (m roundModel) option(d int) focus {
	if m.at.kind == sendButton {
		return m.at
	}
	n := len(m.round.Questions[m.at.q].Options)
	i := n
	if m.at.kind == optionButton {
		i = m.at.opt
	}
	i = max(min(i+d, n), 0)
	if i == n {
		return focus{kind: wordsButton, q: m.at.q}
	}
	return focus{kind: optionButton, q: m.at.q, opt: i}
}

// roundHit is a button's rectangle on screen, cells x0 to x1 (exclusive) on
// rows y0 to y1, and the button a tap there focuses.
type roundHit struct {
	x0, x1, y0, y1 int
	at             focus
}

// frame is the Round laid out: the scrolling body and the footer kept at the
// bottom, each with the hits of its buttons in its own lines. Hits come from
// the same pass that draws, so a tap lands on what is drawn.
func (m roundModel) frame() (body []string, bodyHits []roundHit, foot []string, footHits []roundHit) {
	w := max(m.width, 20)
	add := func(s string, indent int) {
		for _, l := range wrap(s, w-indent) {
			body = append(body, strings.Repeat(" ", indent)+l)
		}
	}
	if head := fmt.Sprintf("%s %s  round %d", m.id, m.title, m.round.N); ansi.StringWidth(head) <= w {
		body = append(body, head)
	} else {
		add(head, 0)
	}
	if m.round.Context != "" {
		body = append(body, "")
		for _, l := range strings.Split(m.round.Context, "\n") {
			add(l, 0)
		}
	}
	body = append(body, strings.Repeat("─", w))

	for qi, q := range m.round.Questions {
		body = append(body, "")
		num := fmt.Sprintf("%d. ", q.N)
		indent := strings.Repeat(" ", len(num))
		for i, l := range wrap(q.Text, w-len(num)) {
			if i == 0 {
				// The number is the question's handle; accent it after the
				// text is wrapped and measured.
				body = append(body, paneStyle.accent(num)+l)
				continue
			}
			body = append(body, indent+l)
		}
		editing := m.editing && m.at.q == qi
		labels := make([]string, 0, len(q.Options)+1)
		for _, o := range q.Options {
			mark := " "
			if m.answers[qi].Letter == o.Letter {
				mark = paneStyle.accent("✓")
			}
			l := mark + o.Letter + " " + o.Text
			if o.Reco {
				l += " " + paneStyle.accent("(reco)")
			}
			labels = append(labels, l)
		}
		plain := " or in plain words…"
		if m.answers[qi].Letter == "" && (editing || m.answers[qi].Text != "") {
			plain = paneStyle.accent("✓") + "or in plain words…"
		}
		labels = append(labels, plain)
		bodyHits = append(bodyHits, m.buttons(&body, labels, qi, w)...)
		if editing || m.answers[qi].Letter == "" && m.answers[qi].Text != "" {
			text := m.answers[qi].Text
			if editing {
				text += "▏"
			}
			bodyHits = append(bodyHits, roundHit{0, w, len(body), len(body), focus{kind: wordsButton, q: qi}})
			body = append(body, "  > "+tail(text, w-4)) // one line: the end being typed stays in view
		}
	}
	body = append(body, "")
	add("tap or ⏎ pick · ↑↓ question · tab words · esc back · q quit", 0)

	if m.status != "" {
		foot = append(foot, wrap(m.status, w)...)
	}
	var marks []string
	for qi, q := range m.round.Questions {
		a := "–"
		switch {
		case m.answers[qi].Letter != "":
			a = m.answers[qi].Letter
		case m.answered(qi):
			a = "✎"
		}
		marks = append(marks, fmt.Sprintf("%d.%s", q.N, a))
	}
	send := focus{kind: sendButton}
	sendBtn := "[ Send ]"
	sendDrawn := sendBtn
	switch {
	case m.at == send:
		sendDrawn = paneStyle.focus(sendBtn)
	case !m.complete():
		sendDrawn = paneStyle.off(sendBtn)
	}
	summary := wrap(strings.Join(marks, " "), w)
	last := summary[len(summary)-1]
	foot = append(foot, summary[:len(summary)-1]...)
	if x := ansi.StringWidth(last) + 3; x+len(sendBtn) <= w {
		footHits = append(footHits, roundHit{x, x + len(sendBtn), len(foot), len(foot), send})
		foot = append(foot, last+"   "+sendDrawn)
	} else {
		foot = append(foot, last)
		footHits = append(footHits, roundHit{0, len(sendBtn), len(foot), len(foot), send})
		foot = append(foot, sendDrawn)
	}
	// The legend sits under the summary that draws those marks, wrapped so
	// a phone never cuts it.
	for _, l := range wrap("✓ picked · ✎ in words · – unanswered", w) {
		foot = append(foot, paneStyle.faint(l))
	}
	return body, bodyHits, foot, footHits
}

// buttons draws question qi's buttons into body and returns their hits:
// all on one line when they fit a screen of inlineWidth or more, else one per
// line, each wrapped to its full text. The last label is plain words.
func (m roundModel) buttons(body *[]string, labels []string, qi, w int) []roundHit {
	var hits []roundHit
	at := func(i int) focus {
		if i == len(labels)-1 {
			return focus{kind: wordsButton, q: qi}
		}
		return focus{kind: optionButton, q: qi, opt: i}
	}
	focused := func(i int, s string) string {
		if !m.editing && m.at == at(i) {
			return paneStyle.focus(s)
		}
		return s
	}
	const indent = "  "
	inline := w >= inlineWidth
	total := len(indent)
	for _, l := range labels {
		total += ansi.StringWidth(l) + 4 // "[" + label + " ]" + gap
	}
	if inline && total-1 <= w {
		var b strings.Builder
		b.WriteString(indent)
		x := len(indent)
		for i, l := range labels {
			s := "[" + l + " ]"
			n := ansi.StringWidth(s)
			hits = append(hits, roundHit{x, x + n, len(*body), len(*body), at(i)})
			b.WriteString(focused(i, s) + " ")
			x += n + 1
		}
		*body = append(*body, strings.TrimRight(b.String(), " "))
		return hits
	}
	for i, l := range labels {
		parts := wrap(strings.TrimLeft("["+l+" ]", " "), w-len(indent)-2)
		y0 := len(*body)
		for j, p := range parts {
			pad := indent
			if j > 0 {
				pad = indent + "  "
			}
			*body = append(*body, pad+focused(i, p))
		}
		hits = append(hits, roundHit{0, w, y0, len(*body) - 1, at(i)})
	}
	return hits
}

// laidOut lays the screen out once for the model's state: the body scrolled
// to top (first moved to show the focused button when follow), the footer
// pinned to the last rows, and every hit moved to screen rows.
func (m roundModel) laidOut(follow bool) roundModel {
	body, bodyHits, foot, footHits := m.frame()
	room := len(body)
	if m.height > 0 {
		room = max(m.height-len(foot), 0)
	}
	if follow {
		for _, h := range bodyHits {
			if h.at != m.at {
				continue
			}
			if h.y0 < m.top {
				m.top = h.y0
			} else if view := max(room, 1); h.y1 >= m.top+view {
				m.top = h.y1 - view + 1
			}
			break
		}
	}
	m.top = max(min(m.top, len(body)-room), 0)
	lines := append([]string{}, body[m.top:min(m.top+room, len(body))]...)
	shown := len(lines)
	var hits []roundHit
	for _, h := range bodyHits {
		h.y0, h.y1 = h.y0-m.top, h.y1-m.top
		if h.y1 >= 0 && h.y0 < shown {
			h.y0, h.y1 = max(h.y0, 0), min(h.y1, shown-1)
			hits = append(hits, h)
		}
	}
	for len(lines) < room {
		lines = append(lines, "")
	}
	for _, h := range footHits {
		h.y0, h.y1 = h.y0+len(lines), h.y1+len(lines)
		hits = append(hits, h)
	}
	m.lines, m.hits = append(lines, foot...), hits
	return m
}

// tail is the end of s in at most n cells, "…" first when it is cut.
func tail(s string, n int) string {
	if ansi.StringWidth(s) <= n {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && ansi.StringWidth(string(r))+1 > n {
		r = r[1:]
	}
	return "…" + string(r)
}

func (m roundModel) View() string { return strings.Join(m.lines, "\n") }
