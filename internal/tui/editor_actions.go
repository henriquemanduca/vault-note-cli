package tui

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// splitLines splits text into logical lines (always at least 1 line).
func splitLines(text string) []string {
	return strings.Split(text, "\n")
}

func clampLineIndex(lines []string, line int) int {
	if line < 0 {
		return 0
	}
	if line >= len(lines) {
		return len(lines) - 1
	}
	return line
}

func clampRuneCol(line string, col int) int {
	n := utf8.RuneCountInString(line)
	if col < 0 {
		return 0
	}
	if col > n {
		return n
	}
	return col
}

// lineAt returns the logical line at index line (or "" if out of range).
func lineAt(text string, line int) string {
	lines := splitLines(text)
	if line < 0 || line >= len(lines) {
		return ""
	}
	return lines[line]
}

// countEditorStats returns word count, rune count, and logical line count.
func countEditorStats(text string) (words, runes, lines int) {
	if text == "" {
		return 0, 0, 1
	}
	words = len(strings.Fields(text))
	runes = utf8.RuneCountInString(text)
	lines = strings.Count(text, "\n") + 1
	return words, runes, lines
}

// leadingWhitespace splits s into leading spaces/tabs and the remaining text.
func leadingWhitespace(s string) (indent, rest string) {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[:i], s[i:]
}

// toggleCheckboxOnLine cycles checklist state on the given logical line:
//   - "- [ ] x" <-> "- [x] x" (also accepts "*" and "[X]")
//   - "- x" / "* x" -> "- [ ] x" / "* [ ] x"
//   - "x" -> "- [ ] x" (preserving indentation)
func toggleCheckboxOnLine(text string, line, col int) (string, int, int) {
	lines := splitLines(text)
	line = clampLineIndex(lines, line)
	cur := lines[line]
	col = clampRuneCol(cur, col)

	indent, rest := leadingWhitespace(cur)
	var updated string
	delta := 0

	switch {
	case strings.HasPrefix(rest, "- [ ] "):
		updated = indent + "- [x] " + rest[len("- [ ] "):]
	case strings.HasPrefix(rest, "- [x] ") || strings.HasPrefix(rest, "- [X] "):
		updated = indent + "- [ ] " + rest[len("- [x] "):]
	case strings.HasPrefix(rest, "* [ ] "):
		updated = indent + "* [x] " + rest[len("* [ ] "):]
	case strings.HasPrefix(rest, "* [x] ") || strings.HasPrefix(rest, "* [X] "):
		updated = indent + "* [ ] " + rest[len("* [x] "):]
	case rest == "- [ ]":
		updated = indent + "- [x]"
	case rest == "- [x]" || rest == "- [X]":
		updated = indent + "- [ ]"
	case rest == "* [ ]":
		updated = indent + "* [x]"
	case rest == "* [x]" || rest == "* [X]":
		updated = indent + "* [ ]"
	case strings.HasPrefix(rest, "- "):
		updated = indent + "- [ ] " + rest[len("- "):]
		delta = 4
	case strings.HasPrefix(rest, "* "):
		updated = indent + "* [ ] " + rest[len("* "):]
		delta = 4
	default:
		updated = indent + "- [ ] " + rest
		delta = 6
	}

	lines[line] = updated
	newCol := clampRuneCol(updated, col+delta)
	return strings.Join(lines, "\n"), line, newCol
}

// parseListPrefix recognizes list markers after leading whitespace and returns
// the matched prefix (including trailing space) and the marker for the next item.
func parseListPrefix(rest string) (prefix, nextPrefix string, ok bool) {
	switch {
	case strings.HasPrefix(rest, "- [ ] "):
		return "- [ ] ", "- [ ] ", true
	case strings.HasPrefix(rest, "- [x] ") || strings.HasPrefix(rest, "- [X] "):
		return rest[:6], "- [ ] ", true
	case strings.HasPrefix(rest, "* [ ] "):
		return "* [ ] ", "* [ ] ", true
	case strings.HasPrefix(rest, "* [x] ") || strings.HasPrefix(rest, "* [X] "):
		return rest[:6], "* [ ] ", true
	case strings.HasPrefix(rest, "- "):
		return "- ", "- ", true
	case strings.HasPrefix(rest, "* "):
		return "* ", "* ", true
	}

	// Ordered list: "<digits>. "
	digits := 0
	for digits < len(rest) && unicode.IsDigit(rune(rest[digits])) {
		digits++
	}
	if digits > 0 && digits+1 < len(rest) && rest[digits] == '.' && rest[digits+1] == ' ' {
		num, err := strconv.Atoi(rest[:digits])
		if err == nil {
			matched := rest[:digits+2]
			return matched, fmt.Sprintf("%d. ", num+1), true
		}
	}
	return "", "", false
}

// continueListOnEnter handles Enter on list/checklist lines:
//   - On an empty item (only indent + marker), clears the marker instead of
//     creating another empty bullet.
//   - Otherwise splits the line at rune col and prepends the continuation marker.
func continueListOnEnter(text string, line, col int) (string, int, int, bool) {
	lines := splitLines(text)
	line = clampLineIndex(lines, line)
	cur := lines[line]
	col = clampRuneCol(cur, col)

	indent, rest := leadingWhitespace(cur)
	matched, nextMarker, ok := parseListPrefix(rest)
	if !ok {
		return text, line, col, false
	}

	fullPrefix := indent + matched
	prefixRunes := utf8.RuneCountInString(fullPrefix)
	if col < prefixRunes {
		return text, line, col, false
	}

	// Empty list item: clear the marker and keep the cursor at column 0.
	remainder := rest[len(matched):]
	if strings.TrimSpace(remainder) == "" && col >= prefixRunes {
		lines[line] = ""
		return strings.Join(lines, "\n"), line, 0, true
	}

	runes := []rune(cur)
	before := string(runes[:col])
	after := string(runes[col:])
	newNextLine := indent + nextMarker + after

	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:line]...)
	out = append(out, before, newNextLine)
	out = append(out, lines[line+1:]...)

	nextCol := utf8.RuneCountInString(indent + nextMarker)
	return strings.Join(out, "\n"), line + 1, nextCol, true
}

// moveLine swaps the logical line at index line with line+delta (-1 up, +1 down).
func moveLine(text string, line, col, delta int) (string, int, int, bool) {
	lines := splitLines(text)
	line = clampLineIndex(lines, line)
	target := line + delta
	if target < 0 || target >= len(lines) {
		return text, line, clampRuneCol(lines[line], col), false
	}
	lines[line], lines[target] = lines[target], lines[line]
	return strings.Join(lines, "\n"), target, clampRuneCol(lines[target], col), true
}

// duplicateLine inserts a copy of the current line immediately below it.
func duplicateLine(text string, line, col int) (string, int, int) {
	lines := splitLines(text)
	line = clampLineIndex(lines, line)
	cur := lines[line]
	col = clampRuneCol(cur, col)

	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:line+1]...)
	out = append(out, cur)
	out = append(out, lines[line+1:]...)
	return strings.Join(out, "\n"), line + 1, col
}

// deleteLine removes the current logical line cleanly.
func deleteLine(text string, line, col int) (string, int, int) {
	lines := splitLines(text)
	line = clampLineIndex(lines, line)
	if len(lines) == 1 {
		return "", 0, 0
	}
	out := make([]string, 0, len(lines)-1)
	out = append(out, lines[:line]...)
	out = append(out, lines[line+1:]...)

	newLine := line
	if newLine >= len(out) {
		newLine = len(out) - 1
	}
	newCol := clampRuneCol(out[newLine], col)
	return strings.Join(out, "\n"), newLine, newCol
}

// indentLine prepends spaces spaces to the current logical line.
func indentLine(text string, line, col, spaces int) (string, int, int) {
	if spaces <= 0 {
		spaces = 2
	}
	lines := splitLines(text)
	line = clampLineIndex(lines, line)
	col = clampRuneCol(lines[line], col)

	lines[line] = strings.Repeat(" ", spaces) + lines[line]
	return strings.Join(lines, "\n"), line, col + spaces
}

// dedentLine removes up to spaces leading spaces (or one leading tab) from line.
func dedentLine(text string, line, col, spaces int) (string, int, int) {
	if spaces <= 0 {
		spaces = 2
	}
	lines := splitLines(text)
	line = clampLineIndex(lines, line)
	cur := lines[line]
	col = clampRuneCol(cur, col)

	removed := 0
	if strings.HasPrefix(cur, "\t") {
		cur = cur[1:]
		removed = 1
	} else {
		for removed < spaces && strings.HasPrefix(cur, " ") {
			cur = cur[1:]
			removed++
		}
	}
	if removed == 0 {
		return text, line, col
	}
	lines[line] = cur
	newCol := col - removed
	if newCol < 0 {
		newCol = 0
	}
	return strings.Join(lines, "\n"), line, newCol
}
