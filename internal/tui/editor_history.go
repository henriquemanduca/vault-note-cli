package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textarea"
)

const (
	defaultHistoryMaxEntries = 100
	typingCoalesceWindow     = 800 * time.Millisecond
)

type editOpKind int

const (
	editOpTyping editOpKind = iota
	editOpDiscrete
)

// editorSnapshot captures the editor text and logical cursor position (line, rune col).
type editorSnapshot struct {
	value string
	line  int
	col   int
}

// editorHistory maintains bounded Undo/Redo stacks with time-based coalescing
// for consecutive single-character typing.
type editorHistory struct {
	undo       []editorSnapshot
	redo       []editorSnapshot
	maxEntries int
	lastEditAt time.Time
	lastOp     editOpKind
	now        func() time.Time
}

func newEditorHistory() editorHistory {
	return editorHistory{
		maxEntries: defaultHistoryMaxEntries,
		now:        time.Now,
	}
}

func (h *editorHistory) clock() time.Time {
	if h.now == nil {
		return time.Now()
	}
	return h.now()
}

func (h *editorHistory) limit() int {
	if h.maxEntries <= 0 {
		return defaultHistoryMaxEntries
	}
	return h.maxEntries
}

// Reset clears both undo and redo stacks (e.g., when switching or reloading a note).
func (h *editorHistory) Reset() {
	h.undo = nil
	h.redo = nil
	h.lastEditAt = time.Time{}
	h.lastOp = editOpDiscrete
}

// RecordBeforeEdit saves snap onto the undo stack prior to mutating the buffer.
// Consecutive editOpTyping calls within typingCoalesceWindow merge into the
// most recent undo checkpoint so typing a word undoes as a single step.
func (h *editorHistory) RecordBeforeEdit(snap editorSnapshot, kind editOpKind) {
	ts := h.clock()
	h.redo = nil

	if kind == editOpTyping &&
		h.lastOp == editOpTyping &&
		!h.lastEditAt.IsZero() &&
		ts.Sub(h.lastEditAt) < typingCoalesceWindow &&
		len(h.undo) > 0 {
		h.lastEditAt = ts
		return
	}

	if len(h.undo) > 0 && h.undo[len(h.undo)-1].value == snap.value {
		h.lastEditAt = ts
		h.lastOp = kind
		return
	}

	h.undo = append(h.undo, snap)
	if max := h.limit(); len(h.undo) > max {
		h.undo = h.undo[len(h.undo)-max:]
	}
	h.lastEditAt = ts
	h.lastOp = kind
}

// Undo pops the previous snapshot and pushes current onto the redo stack.
func (h *editorHistory) Undo(current editorSnapshot) (editorSnapshot, bool) {
	if len(h.undo) == 0 {
		return editorSnapshot{}, false
	}
	prev := h.undo[len(h.undo)-1]
	h.undo = h.undo[:len(h.undo)-1]

	h.redo = append(h.redo, current)
	if max := h.limit(); len(h.redo) > max {
		h.redo = h.redo[len(h.redo)-max:]
	}
	h.lastOp = editOpDiscrete
	h.lastEditAt = time.Time{}
	return prev, true
}

// Redo pops the next snapshot and pushes current onto the undo stack.
func (h *editorHistory) Redo(current editorSnapshot) (editorSnapshot, bool) {
	if len(h.redo) == 0 {
		return editorSnapshot{}, false
	}
	next := h.redo[len(h.redo)-1]
	h.redo = h.redo[:len(h.redo)-1]

	h.undo = append(h.undo, current)
	if max := h.limit(); len(h.undo) > max {
		h.undo = h.undo[len(h.undo)-max:]
	}
	h.lastOp = editOpDiscrete
	h.lastEditAt = time.Time{}
	return next, true
}

// editorCursorPos returns the current 0-based logical line and rune column in ed.
func editorCursorPos(ed textarea.Model) (line, col int) {
	li := ed.LineInfo()
	return ed.Line(), li.StartColumn + li.ColumnOffset
}

// setEditorCursor moves ed's cursor deterministically to (targetLine, targetCol)
// where targetLine is a 0-based logical line index and targetCol is a rune offset.
func setEditorCursor(ed *textarea.Model, targetLine, targetCol int) {
	total := ed.LineCount()
	if total <= 0 {
		return
	}
	if targetLine < 0 {
		targetLine = 0
	}
	if targetLine >= total {
		targetLine = total - 1
	}

	// Safety bound guards against any unexpected wrapping edge case.
	maxSteps := total + 4
	for steps := 0; ed.Line() < targetLine && steps < maxSteps; steps++ {
		before := ed.Line()
		ed.CursorEnd()
		ed.CursorDown()
		if ed.Line() == before {
			break
		}
	}
	for steps := 0; ed.Line() > targetLine && steps < maxSteps; steps++ {
		before := ed.Line()
		ed.CursorStart()
		ed.CursorUp()
		if ed.Line() == before {
			break
		}
	}

	if targetCol < 0 {
		targetCol = 0
	}
	ed.SetCursor(targetCol)
}

func captureEditorSnapshot(ed textarea.Model) editorSnapshot {
	line, col := editorCursorPos(ed)
	return editorSnapshot{
		value: ed.Value(),
		line:  line,
		col:   col,
	}
}

func applyEditorSnapshot(ed *textarea.Model, snap editorSnapshot) {
	ed.SetValue(snap.value)
	setEditorCursor(ed, snap.line, snap.col)
}
