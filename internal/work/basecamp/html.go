package basecamp

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// htmlToMarkdown renders Basecamp's rich text as markdown. Basecamp sends
// HTML and an agent reads markdown, so this is where the two meet.
//
// It covers what Basecamp's own editor emits and passes anything else
// through as its text, which is the right failure: an agent reading plain
// words is better off than one reading angle brackets.
func htmlToMarkdown(src string) string {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return strings.TrimSpace(src)
	}
	var b strings.Builder
	render(&b, doc)
	return tidy(b.String())
}

func render(b *strings.Builder, n *html.Node) {
	switch n.Type {
	case html.TextNode:
		b.WriteString(n.Data)
	case html.ElementNode:
		renderElement(b, n)
	default:
		renderChildren(b, n)
	}
}

func renderChildren(b *strings.Builder, n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		render(b, c)
	}
}

func renderElement(b *strings.Builder, n *html.Node) {
	switch n.Data {
	case "script", "style", "head":
		return
	case "br":
		b.WriteString("\n")
	case "p", "div", "section", "article":
		block(b, n)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		b.WriteString("\n\n" + strings.Repeat("#", int(n.Data[1]-'0')) + " ")
		renderChildren(b, n)
		b.WriteString("\n\n")
	case "strong", "b":
		wrap(b, n, "**")
	case "em", "i":
		wrap(b, n, "*")
	case "del", "s", "strike":
		wrap(b, n, "~~")
	case "code":
		wrap(b, n, "`")
	case "pre":
		b.WriteString("\n\n```\n")
		renderChildren(b, n)
		b.WriteString("\n```\n\n")
	case "blockquote":
		b.WriteString("\n\n")
		quote(b, n)
		b.WriteString("\n\n")
	case "ul":
		b.WriteString("\n\n")
		list(b, n, false)
		b.WriteString("\n\n")
	case "ol":
		b.WriteString("\n\n")
		list(b, n, true)
		b.WriteString("\n\n")
	case "a":
		link(b, n)
	default:
		renderChildren(b, n)
	}
}

// block renders n as its own paragraph, with blank lines around it.
func block(b *strings.Builder, n *html.Node) {
	b.WriteString("\n\n")
	renderChildren(b, n)
	b.WriteString("\n\n")
}

func wrap(b *strings.Builder, n *html.Node, mark string) {
	b.WriteString(mark)
	renderChildren(b, n)
	b.WriteString(mark)
}

// link keeps a link's target, but not when the text is already the target —
// "[https://x](https://x)" is noise.
func link(b *strings.Builder, n *html.Node) {
	var inner strings.Builder
	renderChildren(&inner, n)
	text := strings.TrimSpace(inner.String())
	href := attr(n, "href")
	if href == "" || href == text {
		b.WriteString(text)
		return
	}
	fmt.Fprintf(b, "[%s](%s)", text, href)
}

func quote(b *strings.Builder, n *html.Node) {
	var inner strings.Builder
	renderChildren(&inner, n)
	for _, line := range strings.Split(tidy(inner.String()), "\n") {
		b.WriteString("> " + line + "\n")
	}
}

// list renders the top-level items of a list. A nested list becomes indented
// lines under its parent rather than a nested markdown list: Basecamp's
// step lists are flat, and the extra structure would not survive anyway.
func list(b *strings.Builder, n *html.Node, ordered bool) {
	i := 0
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Data != "li" {
			continue
		}
		i++
		marker := "- "
		if ordered {
			marker = fmt.Sprintf("%d. ", i)
		}
		var item strings.Builder
		renderChildren(&item, c)
		lines := strings.Split(tidy(item.String()), "\n")
		fmt.Fprintf(b, "%s%s\n", marker, lines[0])
		for _, l := range lines[1:] {
			if l != "" {
				fmt.Fprintf(b, "  %s\n", l)
			}
		}
	}
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

// tidy collapses the blank-line noise the block rules leave behind. Two blank
// lines is a paragraph break; three is an accident.
func tidy(s string) string {
	var out []string
	blanks := 0
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			if blanks++; blanks > 1 {
				continue
			}
			line = ""
		} else {
			blanks = 0
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
