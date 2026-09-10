package basecamp

import "testing"

func TestHTMLLeavesMarkdownBehind(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"plain text", "just words", "just words"},
		{"entities", "Ben &amp; Jerry&#39;s", "Ben & Jerry's"},
		{"paragraphs", "<p>one</p><p>two</p>", "one\n\ntwo"},
		{"divs, as Basecamp sends them", "<div>one</div><div>two</div>", "one\n\ntwo"},
		{"bold and italic", "<strong>a</strong> and <em>b</em>", "**a** and *b*"},
		{"line break", "a<br>b", "a\nb"},
		{"link", `<a href="https://x.test">the docs</a>`, "[the docs](https://x.test)"},
		{"a link whose text is the target", `<a href="https://x.test">https://x.test</a>`, "https://x.test"},
		{"inline code", "run <code>go test</code>", "run `go test`"},
		{"code block", "<pre>a\nb</pre>", "```\na\nb\n```"},
		{"heading", "<h2>Steps</h2>", "## Steps"},
		{"unordered list", "<ul><li>one</li><li>two</li></ul>", "- one\n- two"},
		{"ordered list", "<ol><li>one</li><li>two</li></ol>", "1. one\n2. two"},
		{"quote", "<blockquote>a\nb</blockquote>", "> a\n> b"},
		{"script is not prose", "<div>a</div><script>alert(1)</script>", "a"},
		{"style is not prose", "<style>p{color:red}</style><div>a</div>", "a"},
		{"empty", "", ""},
		{"unknown tags keep their words", "<table><tr><td>a</td></tr></table>", "a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := htmlToMarkdown(tc.in); got != tc.want {
				t.Fatalf("htmlToMarkdown(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// A criteria list written as a real Basecamp step list should read as a list,
// not as one run-on line.
func TestHTMLRendersNestedListItemsOnTheirOwnLines(t *testing.T) {
	got := htmlToMarkdown("<ul><li>parent<ul><li>child</li></ul></li></ul>")
	if got != "- parent\n  - child" && got != "- parent\n  child" {
		t.Fatalf("nested list rendered as %q", got)
	}
}
