package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.layout.mode == layoutTooSmall {
		msg := fmt.Sprintf("Terminal muito pequeno (%dx%d)\nMínimo: %dx%d",
			m.layout.width, m.layout.height, minTerminalWidth, minTerminalHeight)
		return lipgloss.Place(m.layout.width, m.layout.height,
			lipgloss.Center, lipgloss.Center, m.styles.muted.Render(msg))
	}

	topBar := m.renderTopBar()
	statusBar := m.renderStatusBar()

	var body string
	if m.layout.mode == layoutNarrow {
		tabs := m.renderNarrowTabs()
		if m.focus == focusList {
			body = lipgloss.JoinVertical(lipgloss.Left, tabs, m.renderLeftPane(m.layout.leftWidth, m.layout.bodyHeight))
		} else {
			body = lipgloss.JoinVertical(lipgloss.Left, tabs, m.renderRightPane(m.layout.rightWidth, m.layout.bodyHeight))
		}
	} else {
		left := m.renderLeftPane(m.layout.leftWidth, m.layout.bodyHeight)
		right := m.renderRightPane(m.layout.rightWidth, m.layout.bodyHeight)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	workspace := lipgloss.JoinVertical(lipgloss.Left, topBar, body, statusBar)

	if m.modal.kind != modalNone {
		return m.overlayModal(workspace)
	}
	return workspace
}

func (m Model) renderTopBar() string {
	left := "vault-note-cli"
	if badges := m.formatHeaderBadges(); badges != "" {
		left += " " + badges
	}
	right := fmt.Sprintf("%d notas", len(m.notes))
	if m.vault == nil {
		right = "vault não configurado"
	} else if len(m.notes) == 1 {
		right = "1 nota"
	}

	innerW := m.layout.width - 2
	if innerW < 10 {
		innerW = 10
	}
	gap := innerW - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		left = truncateDisplay(left, innerW-ansi.StringWidth(right)-1)
		gap = max(1, innerW-ansi.StringWidth(left)-ansi.StringWidth(right))
	}
	content := left + strings.Repeat(" ", gap) + right
	return m.styles.topBar.Width(m.layout.width).Render(content)
}

func (m Model) renderNarrowTabs() string {
	t1 := "[1] Notas"
	t2 := "[2] Editor"
	if m.focus == focusList {
		t1 = m.styles.tabActive.Render(t1)
		t2 = m.styles.tabIdle.Render(t2)
	} else {
		t1 = m.styles.tabIdle.Render(t1)
		t2 = m.styles.tabActive.Render(t2)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, t1, " ", t2)
}

func (m Model) renderLeftPane(outerW, outerH int) string {
	boxW := outerW - 2
	innerH := outerH - 2
	if boxW < 8 {
		boxW = 8
	}
	if innerH < 3 {
		innerH = 3
	}
	contentW := boxW - 2 // horizontal padding (1 left + 1 right)

	header := m.styles.paneTitle.Render(truncateDisplay("Notas", contentW))
	var searchLine string
	if m.searchActive {
		searchLine = m.searchInput.View()
	} else if m.filterQuery != "" {
		searchLine = m.styles.badge.Render(truncateDisplay("/ "+m.filterQuery, contentW))
	} else {
		searchLine = m.styles.subtle.Render(truncateDisplay("/ filtrar...", contentW))
	}

	visible := innerH - 2
	if visible < 1 {
		visible = 1
	}

	lines := make([]string, 0, innerH)
	lines = append(lines, header, searchLine)

	if m.vault == nil {
		lines = append(lines, m.styles.muted.Render(truncateDisplay("Sem vault", contentW)))
		lines = append(lines, m.styles.subtle.Render(truncateDisplay("Pressione 'c'", contentW)))
	} else if len(m.notes) == 0 {
		lines = append(lines, m.styles.muted.Render(truncateDisplay("Nenhuma nota", contentW)))
		lines = append(lines, m.styles.subtle.Render(truncateDisplay("Pressione 'n'", contentW)))
	} else {
		end := min(len(m.notes), m.listOffset+visible)
		for i := m.listOffset; i < end; i++ {
			n := m.notes[i]
			prefix := "  "
			if i == m.selected {
				prefix = "▸ "
			}
			label := prefix + n.Title
			label = truncateDisplay(label, contentW)
			label = padOrTruncatePlain(label, contentW)
			if i == m.selected {
				lines = append(lines, m.styles.itemSelect.Render(label))
			} else {
				lines = append(lines, m.styles.item.Render(label))
			}
		}
	}

	for len(lines) < innerH {
		lines = append(lines, "")
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}

	boxStyle := m.styles.pane
	if m.focus == focusList {
		boxStyle = m.styles.paneFocus
	}
	return boxStyle.Width(boxW).Height(innerH).Render(strings.Join(lines, "\n"))
}

func (m Model) renderRightPane(outerW, outerH int) string {
	boxW := outerW - 2
	innerH := outerH - 2
	if boxW < 12 {
		boxW = 12
	}
	if innerH < 4 {
		innerH = 4
	}
	contentW := boxW - 2 // horizontal padding (1 left + 1 right)

	boxStyle := m.styles.pane
	if m.focus == focusEditor {
		boxStyle = m.styles.paneFocus
	}

	if m.vault == nil {
		empty := lipgloss.Place(contentW, innerH, lipgloss.Center, lipgloss.Center,
			m.styles.muted.Render("Nenhum vault configurado.\nPressione 'c' para definir o caminho do vault."))
		return boxStyle.Width(boxW).Height(innerH).Render(empty)
	}

	n := m.currentNote()
	if n == nil {
		empty := lipgloss.Place(contentW, innerH, lipgloss.Center, lipgloss.Center,
			m.styles.muted.Render("Selecione uma nota ou pressione 'n' para criar um arquivo .txt."))
		return boxStyle.Width(boxW).Height(innerH).Render(empty)
	}

	// Title + status indicator line.
	stateBadge := m.styles.saved.Render("✓ salvo")
	if m.dirty {
		stateBadge = m.styles.dirty.Render("● modificado")
	}
	titleLeft := n.Title
	availTitle := contentW - ansi.StringWidth(stateBadge) - 2
	if availTitle < 8 {
		availTitle = 8
	}
	titleLeft = truncateDisplay(titleLeft, availTitle)
	gap := max(1, contentW-ansi.StringWidth(titleLeft)-ansi.StringWidth(stateBadge))
	titleLine := m.styles.paneTitle.Render(titleLeft) + strings.Repeat(" ", gap) + stateBadge

	// Metadata subline (notebook folder + relative .txt path + modified timestamp).
	nbBadge := "[Raiz]"
	if n.Notebook != "" {
		nbBadge = "[" + n.Notebook + "]"
	}
	metaParts := []string{nbBadge, n.ID}
	if n.UpdatedAt != "" {
		metaParts = append(metaParts, n.UpdatedAt)
	}
	metaLine := m.styles.badge.Render(truncateDisplay(strings.Join(metaParts, "  "), contentW))

	divider := m.styles.subtle.Render(strings.Repeat("─", contentW))

	editorView := m.editor.View()
	content := lipgloss.JoinVertical(lipgloss.Left, titleLine, metaLine, divider, editorView)
	return boxStyle.Width(boxW).Height(innerH).Render(content)
}

func (m Model) renderStatusBar() string {
	var text string
	if m.statusMsg != "" {
		if m.statusErr {
			text = m.styles.errText.Render("✗ " + m.statusMsg)
		} else {
			text = m.styles.saved.Render(m.statusMsg)
		}
	} else if m.focus == focusEditor {
		text = "[Editor] Ctrl+S salvar · Ctrl+E $EDITOR · Esc/Shift+Tab voltar à lista · Ctrl+C sair"
	} else {
		text = "[Lista] j/k mover · Enter/Tab editar · n nova · m mover · E $EDITOR · d lixeira · b cadernos · c config · / busca · ? ajuda · q sair"
	}
	return m.styles.statusBar.Width(m.layout.width).Render(truncateDisplay(text, m.layout.width-2))
}
