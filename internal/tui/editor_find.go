package tui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// textMatch describes one occurrence of a query in logical-line and global-rune
// coordinates. All indices are rune offsets (UTF-8 safe).
type textMatch struct {
	line      int
	startCol  int
	endCol    int
	startRune int
	endRune   int
}

// findState holds the state for the inline Find / Find & Replace bar.
type findState struct {
	active        bool
	replaceMode   bool
	focusReplace  bool
	caseSensitive bool
	queryInput    textinput.Model
	replaceInput  textinput.Model
	matches       []textMatch
	current       int
}

func newFindState(p palette, disableBlink bool) findState {
	qi := textinput.New()
	qi.Prompt = "Buscar: "
	qi.Placeholder = "texto..."
	applyTextInputTheme(&qi, p)

	ri := textinput.New()
	ri.Prompt = "Subst.: "
	ri.Placeholder = "novo texto..."
	applyTextInputTheme(&ri, p)

	if disableBlink {
		_ = qi.Cursor.SetMode(cursor.CursorStatic)
		_ = ri.Cursor.SetMode(cursor.CursorStatic)
	}
	return findState{
		queryInput:   qi,
		replaceInput: ri,
	}
}

func (f findState) barHeight() int {
	if !f.active {
		return 0
	}
	if f.replaceMode {
		return 2
	}
	return 1
}

// findMatchesInText finds all non-overlapping occurrences of query in content
// using strict rune-by-rune comparison so indices never drift on multi-byte runes.
func findMatchesInText(content, query string, caseSensitive bool) []textMatch {
	qRunes := []rune(query)
	if len(qRunes) == 0 {
		return nil
	}
	if !caseSensitive {
		qRunes = lowerRunes(qRunes)
	}

	cRunes := []rune(content)
	cmpRunes := cRunes
	if !caseSensitive {
		cmpRunes = lowerRunes(cRunes)
	}

	// Precompute (line, col) for each rune index only as we scan forward.
	var matches []textMatch
	line := 0
	col := 0
	qLen := len(qRunes)
	limit := len(cmpRunes) - qLen

	for i := 0; i <= limit; {
		if runesEqual(cmpRunes[i:i+qLen], qRunes) {
			endLine := line
			endCol := col
			for k := 0; k < qLen; k++ {
				if cRunes[i+k] == '\n' {
					endLine++
					endCol = 0
				} else {
					endCol++
				}
			}
			matches = append(matches, textMatch{
				line:      line,
				startCol:  col,
				endCol:    endCol,
				startRune: i,
				endRune:   i + qLen,
			})
			line = endLine
			col = endCol
			i += qLen
			continue
		}
		if cRunes[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
		i++
	}
	return matches
}

func lowerRunes(in []rune) []rune {
	out := make([]rune, len(in))
	for i, r := range in {
		out[i] = unicode.ToLower(r)
	}
	return out
}

func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// replaceMatchAt replaces the rune span [match.startRune:match.endRune] with replacement.
func replaceMatchAt(content string, match textMatch, replacement string) string {
	runes := []rune(content)
	if match.startRune < 0 || match.endRune > len(runes) || match.startRune > match.endRune {
		return content
	}
	var b strings.Builder
	b.WriteString(string(runes[:match.startRune]))
	b.WriteString(replacement)
	b.WriteString(string(runes[match.endRune:]))
	return b.String()
}

// replaceAllMatches replaces all non-overlapping occurrences of query with replacement.
func replaceAllMatches(content, query, replacement string, caseSensitive bool) (string, int) {
	matches := findMatchesInText(content, query, caseSensitive)
	if len(matches) == 0 {
		return content, 0
	}
	runes := []rune(content)
	var b strings.Builder
	prev := 0
	for _, m := range matches {
		b.WriteString(string(runes[prev:m.startRune]))
		b.WriteString(replacement)
		prev = m.endRune
	}
	b.WriteString(string(runes[prev:]))
	return b.String(), len(matches)
}

func (m *Model) openFindBar(replaceMode bool) {
	m.find.active = true
	m.find.replaceMode = replaceMode
	m.find.focusReplace = false
	m.find.queryInput.Focus()
	m.find.replaceInput.Blur()
	m.editor.Blur()
	m.syncEditorSize()
	m.refreshFindMatches(true)
}

func (m *Model) closeFindBar() {
	if !m.find.active {
		return
	}
	m.find.active = false
	m.find.focusReplace = false
	m.find.queryInput.Blur()
	m.find.replaceInput.Blur()
	m.syncEditorSize()
	if m.focus == focusEditor {
		m.editor.Focus()
	}
}

func (m *Model) refreshFindMatches(jump bool) {
	q := m.find.queryInput.Value()
	m.find.matches = findMatchesInText(m.editor.Value(), q, m.find.caseSensitive)
	if len(m.find.matches) == 0 {
		m.find.current = 0
		return
	}
	if m.find.current < 0 || m.find.current >= len(m.find.matches) {
		m.find.current = 0
	}
	if jump {
		m.jumpToCurrentMatch()
	}
}

func (m *Model) jumpToCurrentMatch() {
	if len(m.find.matches) == 0 {
		return
	}
	match := m.find.matches[m.find.current]
	setEditorCursor(&m.editor, match.line, match.startCol)
}

func (m *Model) stepFindMatch(delta int) {
	if len(m.find.matches) == 0 {
		return
	}
	n := len(m.find.matches)
	m.find.current = (m.find.current + delta%n + n) % n
	m.jumpToCurrentMatch()
}

func (m *Model) syncFindFocus() {
	if m.find.replaceMode && m.find.focusReplace {
		m.find.queryInput.Blur()
		m.find.replaceInput.Focus()
	} else {
		m.find.replaceInput.Blur()
		m.find.queryInput.Focus()
	}
}

func (m Model) updateEditorFind(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		m.saveOnQuit()
		return m, tea.Quit

	case "esc":
		m.closeFindBar()
		return m, nil

	case "ctrl+f":
		m.find.replaceMode = false
		m.find.focusReplace = false
		m.syncFindFocus()
		m.syncEditorSize()
		return m, nil

	case "ctrl+r":
		m.find.replaceMode = true
		m.find.focusReplace = true
		m.syncFindFocus()
		m.syncEditorSize()
		return m, nil

	case "tab", "shift+tab":
		if m.find.replaceMode {
			m.find.focusReplace = !m.find.focusReplace
			m.syncFindFocus()
		}
		return m, nil

	case "alt+c":
		m.find.caseSensitive = !m.find.caseSensitive
		m.refreshFindMatches(true)
		return m, nil

	case "f3", "ctrl+g":
		m.stepFindMatch(1)
		return m, nil

	case "shift+f3", "alt+g":
		m.stepFindMatch(-1)
		return m, nil

	case "enter":
		if m.find.replaceMode && m.find.focusReplace {
			m.replaceCurrentMatch()
			return m, nil
		}
		m.stepFindMatch(1)
		return m, nil

	case "alt+r":
		if m.find.replaceMode {
			m.replaceCurrentMatch()
		}
		return m, nil

	case "alt+a":
		if m.find.replaceMode {
			m.replaceAllCurrentMatches()
		}
		return m, nil
	}

	var cmd tea.Cmd
	if m.find.replaceMode && m.find.focusReplace {
		m.find.replaceInput, cmd = m.find.replaceInput.Update(msg)
		return m, cmd
	}
	prevQuery := m.find.queryInput.Value()
	m.find.queryInput, cmd = m.find.queryInput.Update(msg)
	if m.find.queryInput.Value() != prevQuery {
		m.find.current = 0
		m.refreshFindMatches(true)
	}
	return m, cmd
}

func (m *Model) replaceCurrentMatch() {
	if len(m.find.matches) == 0 {
		m.setStatus("Nenhuma ocorrência para substituir", false)
		return
	}
	match := m.find.matches[m.find.current]
	snap := captureEditorSnapshot(m.editor)
	m.history.RecordBeforeEdit(snap, editOpDiscrete)

	updated := replaceMatchAt(m.editor.Value(), match, m.find.replaceInput.Value())
	m.editor.SetValue(updated)
	setEditorCursor(&m.editor, match.line, match.startCol+len([]rune(m.find.replaceInput.Value())))
	m.dirty = m.editor.Value() != m.loadedContent
	m.refreshFindMatches(true)
	m.setStatus("Ocorrência substituída", false)
}

func (m *Model) replaceAllCurrentMatches() {
	query := m.find.queryInput.Value()
	if query == "" {
		return
	}
	updated, count := replaceAllMatches(m.editor.Value(), query, m.find.replaceInput.Value(), m.find.caseSensitive)
	if count == 0 {
		m.setStatus("Nenhuma ocorrência encontrada", false)
		return
	}
	snap := captureEditorSnapshot(m.editor)
	m.history.RecordBeforeEdit(snap, editOpDiscrete)

	m.editor.SetValue(updated)
	setEditorCursor(&m.editor, snap.line, snap.col)
	m.dirty = m.editor.Value() != m.loadedContent
	m.refreshFindMatches(false)
	m.setStatus(fmt.Sprintf("%d substituição(ões) aplicada(s)", count), false)
}

func (m Model) renderFindBar(width int) string {
	counter := "0/0"
	if len(m.find.matches) > 0 {
		counter = fmt.Sprintf("%d/%d", m.find.current+1, len(m.find.matches))
	}
	caseBadge := "[aa]"
	if m.find.caseSensitive {
		caseBadge = "[Aa]"
	}
	info := m.styles.badge.Render(counter+" "+caseBadge) + " " + m.styles.subtle.Render("Enter: próx · Alt+C: Aa · Esc")
	row1 := lipgloss.JoinHorizontal(lipgloss.Top, m.find.queryInput.View(), "  ", info)
	row1 = truncateDisplay(row1, width)

	if !m.find.replaceMode {
		return row1
	}
	repInfo := m.styles.subtle.Render("Enter/Alt+R: subst · Alt+A: todas · Tab")
	row2 := lipgloss.JoinHorizontal(lipgloss.Top, m.find.replaceInput.View(), "  ", repInfo)
	row2 = truncateDisplay(row2, width)
	return lipgloss.JoinVertical(lipgloss.Left, row1, row2)
}
