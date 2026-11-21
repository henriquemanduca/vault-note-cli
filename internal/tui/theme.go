package tui

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// palette centralizes all color tokens using AdaptiveColor so both light and
// dark terminals have high contrast with neutral (achromatic) chrome and
// desaturated semantic accents.
type palette struct {
	// Achromatic chrome tokens (R == G == B).
	Fg          lipgloss.AdaptiveColor
	FgMuted     lipgloss.AdaptiveColor
	FgSubtle    lipgloss.AdaptiveColor
	Border      lipgloss.AdaptiveColor
	BorderFocus lipgloss.AdaptiveColor
	Surface     lipgloss.AdaptiveColor
	HeaderBg    lipgloss.AdaptiveColor
	HeaderFg    lipgloss.AdaptiveColor
	SelectBg    lipgloss.AdaptiveColor
	SelectFg    lipgloss.AdaptiveColor
	ScrimFg     lipgloss.AdaptiveColor
	ModalLabel  lipgloss.AdaptiveColor

	// Desaturated semantic accents.
	Saved lipgloss.AdaptiveColor
	Dirty lipgloss.AdaptiveColor
	Error lipgloss.AdaptiveColor
	Badge lipgloss.AdaptiveColor
}

func graphitePalette() palette {
	return palette{
		Fg:          lipgloss.AdaptiveColor{Light: "#1F1F1F", Dark: "#E4E4E4"},
		FgMuted:     lipgloss.AdaptiveColor{Light: "#5A5A5A", Dark: "#9E9E9E"},
		FgSubtle:    lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#666666"},
		Border:      lipgloss.AdaptiveColor{Light: "#C6C6C6", Dark: "#3D3D3D"},
		BorderFocus: lipgloss.AdaptiveColor{Light: "#2E2E2E", Dark: "#D4D4D4"},
		Surface:     lipgloss.AdaptiveColor{Light: "#F5F5F5", Dark: "#1C1C1C"},
		HeaderBg:    lipgloss.AdaptiveColor{Light: "#E8E8E8", Dark: "#262626"},
		HeaderFg:    lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#EBEBEB"},
		SelectBg:    lipgloss.AdaptiveColor{Light: "#DCDCDC", Dark: "#343434"},
		SelectFg:    lipgloss.AdaptiveColor{Light: "#121212", Dark: "#F2F2F2"},
		ScrimFg:     lipgloss.AdaptiveColor{Light: "#A8A8A8", Dark: "#525252"},
		ModalLabel:  lipgloss.AdaptiveColor{Light: "#272727", Dark: "#E4E4E4"},

		Saved: lipgloss.AdaptiveColor{Light: "#2F6F4E", Dark: "#73A88A"},
		Dirty: lipgloss.AdaptiveColor{Light: "#9A6728", Dark: "#CCA06B"},
		Error: lipgloss.AdaptiveColor{Light: "#A8383B", Dark: "#D47A7A"},
		Badge: lipgloss.AdaptiveColor{Light: "#4A607A", Dark: "#8DA4BF"},
	}
}

type styles struct {
	p palette

	topBar      lipgloss.Style
	statusBar   lipgloss.Style
	pane        lipgloss.Style
	paneFocus   lipgloss.Style
	paneTitle   lipgloss.Style
	item        lipgloss.Style
	itemSelect  lipgloss.Style
	normalRow   lipgloss.Style
	accent      lipgloss.Style
	muted       lipgloss.Style
	subtle      lipgloss.Style
	saved       lipgloss.Style
	dirty       lipgloss.Style
	errText     lipgloss.Style
	abortBanner lipgloss.Style
	badge       lipgloss.Style
	tabActive   lipgloss.Style
	tabIdle     lipgloss.Style
	scrim       lipgloss.Style
	modalBox    lipgloss.Style
	modalTitle  lipgloss.Style
	modalLabel  lipgloss.Style
	modalHint   lipgloss.Style
}

func newStyles(p palette) styles {
	return styles{
		p: p,
		topBar: lipgloss.NewStyle().
			Background(p.HeaderBg).
			Foreground(p.HeaderFg).
			Bold(true).
			Padding(0, 1).
			MarginTop(1),
		statusBar: lipgloss.NewStyle().
			Background(p.HeaderBg).
			Foreground(p.FgMuted).
			Padding(0, 1),
		pane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.Border).
			Padding(0, 1),
		paneFocus: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.BorderFocus).
			Padding(0, 1),
		paneTitle: lipgloss.NewStyle().
			Foreground(p.Fg).
			Bold(true),
		item: lipgloss.NewStyle().
			Foreground(p.Fg),
		itemSelect: lipgloss.NewStyle().
			Background(p.SelectBg).
			Foreground(p.SelectFg).
			Bold(true),
		normalRow: lipgloss.NewStyle(),
		accent: lipgloss.NewStyle().
			Foreground(p.BorderFocus).
			Bold(true),
		muted: lipgloss.NewStyle().
			Foreground(p.FgMuted),
		subtle: lipgloss.NewStyle().
			Foreground(p.FgSubtle),
		saved: lipgloss.NewStyle().
			Foreground(p.Saved),
		dirty: lipgloss.NewStyle().
			Foreground(p.Dirty).
			Bold(true),
		errText: lipgloss.NewStyle().
			Foreground(p.Error).
			Bold(true),
		abortBanner: lipgloss.NewStyle().
			Foreground(p.Error).
			Bold(true),
		badge: lipgloss.NewStyle().
			Foreground(p.Badge),
		tabActive: lipgloss.NewStyle().
			Background(p.SelectBg).
			Foreground(p.SelectFg).
			Bold(true).
			Padding(0, 1),
		tabIdle: lipgloss.NewStyle().
			Foreground(p.FgMuted).
			Padding(0, 1),
		scrim: lipgloss.NewStyle().
			Foreground(p.ScrimFg),
		modalBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.BorderFocus).
			Padding(1, 2),
		modalTitle: lipgloss.NewStyle().
			Foreground(p.BorderFocus).
			Bold(true),
		modalLabel: lipgloss.NewStyle().
			Foreground(p.ModalLabel).
			Bold(true),
		modalHint: lipgloss.NewStyle().
			Foreground(p.FgMuted),
	}
}

func applyTextInputTheme(ti *textinput.Model, p palette) {
	ti.PromptStyle = lipgloss.NewStyle().Foreground(p.BorderFocus).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(p.Fg)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(p.FgMuted)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(p.BorderFocus)
}

func applyTextareaTheme(ta *textarea.Model, p palette) {
	ta.FocusedStyle.Base = lipgloss.NewStyle().Foreground(p.Fg)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(p.HeaderBg)
	ta.FocusedStyle.LineNumber = lipgloss.NewStyle().Foreground(p.FgSubtle)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(p.BorderFocus)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(p.FgSubtle)
	ta.FocusedStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(p.FgSubtle)

	ta.BlurredStyle.Base = lipgloss.NewStyle().Foreground(p.Fg)
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.LineNumber = lipgloss.NewStyle().Foreground(p.FgSubtle)
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(p.Border)
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(p.FgSubtle)
	ta.BlurredStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(p.FgSubtle)
}
