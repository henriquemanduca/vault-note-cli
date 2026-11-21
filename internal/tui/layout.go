package tui

const (
	minTerminalWidth  = 36
	minTerminalHeight = 12
	narrowBreakpoint  = 70
	minSidebarWidth   = 20
)

type layoutMode int

const (
	layoutTooSmall layoutMode = iota
	layoutNarrow
	layoutMasterDetail
)

type layoutMetrics struct {
	mode       layoutMode
	width      int
	height     int
	bodyHeight int
	leftWidth  int // outer width including border
	rightWidth int // outer width including border
}

// computeLayout resolves responsive breakpoints and enforces the 20% left / 80%
// right master-detail split for standard terminals (>= 70 cols).
func computeLayout(width, height int) layoutMetrics {
	if width < minTerminalWidth || height < minTerminalHeight {
		return layoutMetrics{
			mode:   layoutTooSmall,
			width:  width,
			height: height,
		}
	}

	if width < narrowBreakpoint {
		// Top margin (1) + top bar (1) + tab bar (1) + status bar (1) = 4 lines reserved.
		bodyH := height - 4
		if bodyH < 6 {
			bodyH = 6
		}
		return layoutMetrics{
			mode:       layoutNarrow,
			width:      width,
			height:     height,
			bodyHeight: bodyH,
			leftWidth:  width,
			rightWidth: width,
		}
	}

	// Master-detail: 20% left pane (minimum 20 cols for legibility), 80% right pane.
	leftW := width * 20 / 100
	if leftW < minSidebarWidth {
		leftW = minSidebarWidth
	}
	rightW := width - leftW
	if rightW < 24 {
		rightW = 24
		leftW = width - rightW
	}

	// Top margin (1) + top bar (1) + status bar (1) = 3 lines reserved.
	bodyH := height - 3
	if bodyH < 6 {
		bodyH = 6
	}

	return layoutMetrics{
		mode:       layoutMasterDetail,
		width:      width,
		height:     height,
		bodyHeight: bodyH,
		leftWidth:  leftW,
		rightWidth: rightW,
	}
}
