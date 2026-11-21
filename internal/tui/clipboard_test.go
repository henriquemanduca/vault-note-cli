package tui

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCopyToClipboardOSC52AndMultiplexers(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("STY", "")
	var buf bytes.Buffer
	if err := copyToClipboardOSC52(&buf, "hello"); err != nil {
		t.Fatalf("copyToClipboardOSC52: %v", err)
	}
	// Standard OSC 52 begins with ESC]52;c; and contains base64("hello") == "aGVsbG8=".
	if !strings.HasPrefix(buf.String(), "\x1b]52;c;") || !strings.Contains(buf.String(), "aGVsbG8=") {
		t.Fatalf("unexpected OSC 52 sequence: %q", buf.String())
	}

	// TMUX passthrough wraps in DCS ESC P tmux;...
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
	buf.Reset()
	if err := copyToClipboardOSC52(&buf, "hello"); err != nil {
		t.Fatalf("copyToClipboardOSC52 tmux: %v", err)
	}
	if !strings.HasPrefix(buf.String(), "\x1bPtmux;") {
		t.Fatalf("expected tmux passthrough prefix, got %q", buf.String())
	}

	// GNU Screen passthrough wraps in DCS ESC P ...
	t.Setenv("TMUX", "")
	t.Setenv("STY", "1234.pts-0")
	buf.Reset()
	if err := copyToClipboardOSC52(&buf, "hello"); err != nil {
		t.Fatalf("copyToClipboardOSC52 screen: %v", err)
	}
	if !strings.HasPrefix(buf.String(), "\x1bP") {
		t.Fatalf("expected screen passthrough prefix, got %q", buf.String())
	}
}

func TestTUIClipboardShortcuts(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("STY", "")
	store, v := newTestEnv(t)
	if _, err := v.CreateNote("", "Clip", "primeira linha\nsegunda linha"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	var clipBuf bytes.Buffer
	m := NewModel(store, v, Options{DisableCursorBlink: true, ClipboardWriter: &clipBuf})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	// Y on list copies full note content.
	m = sendKey(t, m, "Y")
	if clipBuf.Len() == 0 || !strings.Contains(m.statusMsg, "copiado") {
		t.Fatalf("expected clipboard output and status, got len=%d status=%q", clipBuf.Len(), m.statusMsg)
	}

	// Alt+Y in editor copies current line ("segunda linha").
	clipBuf.Reset()
	m = sendKey(t, m, "tab")
	m = sendKey(t, m, "alt+y")
	if !strings.Contains(clipBuf.String(), "c2VndW5kYSBsaW5oYQ==") { // base64("segunda linha")
		t.Fatalf("expected current line copied, got %q", clipBuf.String())
	}

	// Alt+W in editor copies full note buffer.
	clipBuf.Reset()
	m = sendKey(t, m, "alt+w")
	if !strings.Contains(clipBuf.String(), "cHJpbWVpcmEgbGluaGEKc2VndW5kYSBsaW5oYQ==") {
		t.Fatalf("expected full buffer copied, got %q", clipBuf.String())
	}
}
