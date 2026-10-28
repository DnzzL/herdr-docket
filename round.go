package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/DnzzL/herdr-docket/internal/grill"
)

// inlineWidth is the narrowest screen a question's buttons may share a line
// on; below it each button gets its own line, so a thumb hits one at a time.
const inlineWidth = 60

// roundDoneMsg leaves the Round for Home, with the line Home shows: the
// confirmation of a sent answer, or nothing on Esc.
type roundDoneMsg struct{ status string }

// answer is one question's answer: an option letter, or free text when
// letter is empty.
type answer struct{ letter, text string }

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
	answers       []answer
	q, opt        int  // focus: q == len(questions) is Send; opt == len(options) is plain words
	editing       bool // the focused question's text field takes the keys
	width, height int
	top           int
	status        string
}

func newRound(id, title string, round grill.Round, send func(string) (string, error), width, height int) roundModel {
	m := roundModel{id: id, title: title, round: round, send: send, width: width, height: height}
	m.answers = make([]answer, len(round.Questions))
	for i, q := range round.Questions {
		for _, o := range q.Options {
			if o.Reco {
				m.answers[i].letter = o.Letter
			}
		}
	}
	m.opt = m.picked(0)
	return m
}

// picked is the button index of question q's answer: its option, else plain
// words.
func (m roundModel) picked(q int) int {
	if q >= len(m.round.Questions) {
		return 0
	}
	opts := m.round.Questions[q].Options
	for i, o := range opts {
		if o.Letter == m.answers[q].letter {
			return i
		}
	}
	if m.answers[q].letter == "" && m.answers[q].text == "" && len(opts) > 0 {
		return 0
	}
	return len(opts)
}

func (m roundModel) answered(q int) bool {
	return m.answers[q].letter != "" || strings.TrimSpace(m.answers[q].text) != ""
}

func (m roundModel) complete() bool {
	for q := range m.answers {
		if !m.answered(q) {
			return false
		}
	}
	return true
}

// line is the answer as the line prompt builds it: `1.A 2.B 3. free text`.
func (m roundModel) line() string {
	parts := make([]string, len(m.answers))
	for i, a := range m.answers {
		n := m.round.Questions[i].N
		if a.letter != "" {
			parts[i] = fmt.Sprintf("%d.%s", n, a.letter)
		} else {
			parts[i] = fmt.Sprintf("%d. %s", n, strings.TrimSpace(a.text))
		}
	}
	return strings.Join(parts, " ")
}

func (m roundModel) Update(msg tea.Msg) (roundModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.MouseMsg:
		return m.mouse(msg)
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m.clamped(), nil
}

func (m roundModel) mouse(msg tea.MouseMsg) (roundModel, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelDown:
		m.top += 2
	case tea.MouseButtonWheelUp:
		m.top -= 2
	case tea.MouseButtonLeft:
		_, hits := m.screen()
		for _, h := range hits {
			if msg.Y >= h.y0 && msg.Y <= h.y1 && msg.X >= h.x0 && msg.X < h.x1 {
				m.q, m.opt = h.q, h.opt
				return m.press()
			}
		}
	}
	return m.clamped(), nil
}

// press is Enter, or a tap, on the focused element.
func (m roundModel) press() (roundModel, tea.Cmd) {
	if m.q >= len(m.round.Questions) {
		return m.submit()
	}
	opts := m.round.Questions[m.q].Options
	if m.opt < len(opts) {
		m.answers[m.q] = answer{letter: opts[m.opt].Letter, text: m.answers[m.q].text}
		m.editing = false
		return m, nil
	}
	m.answers[m.q].letter = ""
	m.editing = !m.editing
	return m, nil
}

func (m roundModel) submit() (roundModel, tea.Cmd) {
	if !m.complete() {
		m.status = "answer every question first"
		return m, nil
	}
	out, err := m.send(m.line())
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	status := strings.TrimSpace(out)
	return m, func() tea.Msg { return roundDoneMsg{status: status} }
}

func (m roundModel) key(msg tea.KeyMsg) (roundModel, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	if m.editing {
		a := &m.answers[m.q]
		switch msg.Type {
		case tea.KeyEnter, tea.KeyTab, tea.KeyEsc:
			m.editing = false
		case tea.KeyBackspace:
			if r := []rune(a.text); len(r) > 0 {
				a.text = string(r[:len(r)-1])
			}
		case tea.KeySpace:
			a.text += " "
		case tea.KeyRunes:
			a.text += strings.NewReplacer("\n", " ", "\r", " ").Replace(string(msg.Runes))
		}
		return m, nil
	}
	nq := len(m.round.Questions)
	switch k {
	case "q":
		return m, tea.Quit
	case "esc":
		return m, func() tea.Msg { return roundDoneMsg{} }
	case "up", "k":
		m.q = max(m.q-1, 0)
		m.opt = m.picked(m.q)
	case "down", "j":
		m.q = min(m.q+1, nq)
		m.opt = m.picked(m.q)
	case "left", "h":
		m.opt = max(m.opt-1, 0)
	case "right", "l":
		if m.q < nq {
			m.opt = min(m.opt+1, len(m.round.Questions[m.q].Options))
		}
	case "enter":
		return m.press()
	case "tab":
		if m.q < nq {
			m.opt = len(m.round.Questions[m.q].Options)
			return m.press()
		}
	default:
		if m.q < nq && len(k) == 1 {
			for i, o := range m.round.Questions[m.q].Options {
				if strings.EqualFold(k, o.Letter) {
					m.opt = i
					return m.press()
				}
			}
		}
	}
	m = m.clamped()
	m.follow()
	return m, nil
}

// roundHit is a button's rectangle on screen, cells x0 to x1 (exclusive) on
// rows y0 to y1, and what a tap there focuses: question q (len(questions)
// for Send) and button opt.
type roundHit struct{ x0, x1, y0, y1, q, opt int }

var (
	focusStyle = lipgloss.NewStyle().Reverse(true)
	offStyle   = lipgloss.NewStyle().Faint(true)
)

// frame is the Round laid out once: the scrolling body and the footer kept
// at the bottom, each with the hits of its buttons in its own lines. Hits
// come from the same pass that draws, so a tap lands on what is drawn.
func (m roundModel) frame() (body []string, bodyHits []roundHit, foot []string, footHits []roundHit) {
	w := max(m.width, 20)
	if m.width <= 0 {
		w = 80
	}
	add := func(s string, indent int) {
		for _, l := range wrap(s, w-indent) {
			body = append(body, strings.Repeat(" ", indent)+l)
		}
	}
	if head := fmt.Sprintf("%s %s  round %d", m.id, m.title, m.round.N); len([]rune(head)) <= w {
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
		add(fmt.Sprintf("%d. %s", q.N, q.Text), 0)
		labels := make([]string, 0, len(q.Options)+1)
		for _, o := range q.Options {
			mark := " "
			if m.answers[qi].letter == o.Letter {
				mark = "✓"
			}
			l := mark + o.Letter + " " + o.Text
			if o.Reco {
				l += " (reco)"
			}
			labels = append(labels, l)
		}
		plain := " or in plain words…"
		if m.answers[qi].letter == "" && (m.editing && m.q == qi || m.answers[qi].text != "") {
			plain = "✓or in plain words…"
		}
		labels = append(labels, plain)
		bodyHits = append(bodyHits, m.buttons(&body, labels, qi, w)...)
		if m.editing && m.q == qi || m.answers[qi].letter == "" && m.answers[qi].text != "" {
			text := m.answers[qi].text
			if m.editing && m.q == qi {
				text += "▏"
			}
			r := []rune(text)
			if room := w - 4; len(r) > room { // one line: the tail being typed stays in view
				r = append([]rune("…"), r[len(r)-room+1:]...)
			}
			bodyHits = append(bodyHits, roundHit{0, w, len(body), len(body), qi, len(q.Options)})
			body = append(body, "  > "+string(r))
		}
	}
	body = append(body, "")
	add("tap or ⏎ pick · ↑↓ question · tab words · esc back · q quit", 0)

	if m.status != "" {
		for _, l := range wrap(m.status, w) {
			foot = append(foot, l)
		}
	}
	var marks []string
	for qi, q := range m.round.Questions {
		a := "–"
		switch {
		case m.answers[qi].letter != "":
			a = m.answers[qi].letter
		case m.answered(qi):
			a = "✎"
		}
		marks = append(marks, fmt.Sprintf("%d.%s", q.N, a))
	}
	sendBtn := "[ Send ]"
	sendDrawn := sendBtn
	switch {
	case m.q == len(m.round.Questions):
		sendDrawn = focusStyle.Render(sendBtn)
	case !m.complete():
		sendDrawn = offStyle.Render(sendBtn)
	}
	summary := wrap(strings.Join(marks, " "), w)
	last := summary[len(summary)-1]
	foot = append(foot, summary[:len(summary)-1]...)
	if len([]rune(last))+3+len(sendBtn) <= w {
		footHits = append(footHits, roundHit{len([]rune(last)) + 3, len([]rune(last)) + 3 + len(sendBtn), len(foot), len(foot), len(m.round.Questions), 0})
		foot = append(foot, last+"   "+sendDrawn)
	} else {
		foot = append(foot, last)
		footHits = append(footHits, roundHit{0, len(sendBtn), len(foot), len(foot), len(m.round.Questions), 0})
		foot = append(foot, sendDrawn)
	}
	return body, bodyHits, foot, footHits
}

// buttons draws question qi's buttons into body and returns their hits:
// all on one line when they fit a screen of inlineWidth or more, else one per
// line, each wrapped to its full text.
func (m roundModel) buttons(body *[]string, labels []string, qi, w int) []roundHit {
	var hits []roundHit
	focus := func(i int, s string) string {
		if !m.editing && m.q == qi && m.opt == i {
			return focusStyle.Render(s)
		}
		return s
	}
	const indent = "  "
	inline := w >= inlineWidth
	total := len(indent)
	for _, l := range labels {
		total += len([]rune(l)) + 4 // "[" + label + " ]" + gap
	}
	if inline && total-1 <= w {
		var b strings.Builder
		b.WriteString(indent)
		x := len(indent)
		for i, l := range labels {
			s := "[" + l + " ]"
			n := len([]rune(s))
			hits = append(hits, roundHit{x, x + n, len(*body), len(*body), qi, i})
			b.WriteString(focus(i, s) + " ")
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
			*body = append(*body, pad+focus(i, p))
		}
		hits = append(hits, roundHit{0, w, y0, len(*body) - 1, qi, i})
	}
	return hits
}

// screen is what View shows, body scrolled to top and the footer pinned to
// the last rows, with every hit moved to screen rows.
func (m roundModel) screen() ([]string, []roundHit) {
	body, bodyHits, foot, footHits := m.frame()
	room := len(body)
	if m.height > 0 {
		room = max(m.height-len(foot), 0)
	}
	top := max(min(m.top, len(body)-room), 0)
	shown := body[top:min(top+room, len(body))]
	var hits []roundHit
	for _, h := range bodyHits {
		h.y0, h.y1 = h.y0-top, h.y1-top
		if h.y1 >= 0 && h.y0 < len(shown) {
			h.y0, h.y1 = max(h.y0, 0), min(h.y1, len(shown)-1)
			hits = append(hits, h)
		}
	}
	lines := append([]string{}, shown...)
	for len(lines) < room {
		lines = append(lines, "")
	}
	for _, h := range footHits {
		h.y0, h.y1 = h.y0+len(lines), h.y1+len(lines)
		hits = append(hits, h)
	}
	return append(lines, foot...), hits
}

// clamped keeps the scroll inside the body.
func (m roundModel) clamped() roundModel {
	body, _, foot, _ := m.frame()
	room := len(body)
	if m.height > 0 {
		room = max(m.height-len(foot), 0)
	}
	m.top = max(min(m.top, len(body)-room), 0)
	return m
}

// follow scrolls the focused button into view after a key moved it.
func (m *roundModel) follow() {
	body, hits, foot, _ := m.frame()
	if m.height <= 0 {
		return
	}
	room := max(m.height-len(foot), 1)
	for _, h := range hits {
		if h.q != m.q || h.opt != m.opt {
			continue
		}
		if h.y0 < m.top {
			m.top = h.y0
		} else if h.y1 >= m.top+room {
			m.top = h.y1 - room + 1
		}
		m.top = max(min(m.top, len(body)-room), 0)
		return
	}
}

func (m roundModel) View() string {
	lines, _ := m.screen()
	return strings.Join(lines, "\n")
}
