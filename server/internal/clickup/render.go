package clickup

import (
	"fmt"
	"strings"
)

// RenderTodo is the line for a mention in the daily note of noteDate (YYYY-MM-DD), without the
// "- [ ]" prefix and the marker. The time shows the date too when the comment is from another day.
// Everything that comes from ClickUp is escaped: the note is rendered as HTML in the dashboard.
func RenderTodo(m Mention, noteDate string) string {
	icon, verb := "💬", "te mencionó en"
	switch m.Reason {
	case ReasonThread:
		icon, verb = "↩️", "respondió en tu hilo de"
	case ReasonTicket:
		icon, verb = "📝", "comentó en tu ticket"
	}
	author := m.Author
	if author == "" {
		author = "Alguien"
	}
	task := m.TaskName
	if task == "" {
		task = m.TaskID
	}

	when := m.Date.Local()
	stamp := when.Format("15:04")
	if when.Format("2006-01-02") != noteDate {
		stamp = when.Format("02/01 15:04")
	}

	line := fmt.Sprintf("%s **%s** %s [%s](%s)", icon, EscapeMarkdown(author), verb, EscapeMarkdown(task), TaskURL(m.TaskID))
	if m.Text != "" {
		line += ": «" + EscapeMarkdown(m.Text) + "»"
	}
	return line + " · " + stamp
}

var markdownEscaper = strings.NewReplacer(
	// HTML first: the dashboard renders the note with marked straight into innerHTML.
	"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;",
	// Markdown syntax that could turn text into links, emphasis or code.
	`\`, `\\`, "`", "\\`", "*", `\*`, "_", `\_`, "[", `\[`, "]", `\]`, "(", `\(`, ")", `\)`,
	"~", `\~`, "|", `\|`, "!", `\!`, "#", `\#`,
	"\r", " ", "\n", " ",
)

// EscapeMarkdown makes untrusted text safe to embed inline in the daily note (rendered as HTML).
func EscapeMarkdown(s string) string {
	return markdownEscaper.Replace(s)
}

var inlineEscaper = strings.NewReplacer(
	`\`, `\\`, "`", "\\`", "*", `\*`, "_", `\_`, "[", `\[`, "]", `\]`,
	"\r", " ", "\n", " ",
)

// EscapeInline escapes only the Markdown syntax that could break a link label or emphasis. It is
// for plain Markdown read as text (e.g. MCP tool output), where HTML entities would just be noise.
func EscapeInline(s string) string {
	return inlineEscaper.Replace(s)
}
