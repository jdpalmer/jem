package runtime

import (
	"github.com/jdpalmer/jem/killring"
	"github.com/jdpalmer/jem/window"
	"strings"
	"testing"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/mode"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/tools"
)

// TestEditor is a headless editor instance for integration tests.
type TestEditor struct {
	t *testing.T
	e *App
}

// NewTestEditor resets package globals and returns a fresh headless runtime.
func NewTestEditor(t *testing.T) *TestEditor {
	t.Helper()
	e := resetTestEditorState()
	VarsInit()
	display.DisplayInitHeadless(24, 80)
	InitCommands()
	tools.InitBackgroundJobs()
	AppInit("test")
	if window.Active.CurrentWindow != nil {
		window.Active.CurrentWindow.Height = uint32(term.Rows())
	}
	return &TestEditor{t: t, e: e}
}

func resetTestEditorState() *App {
	e := New()
	e.Activate()
	killring.ResetForTests()
	ClearPending()
	clearListeners()
	// Drain any leftover events from a prior test.
	event.DrainForTest()
	tools.ResetBackgroundJobsForTests()
	return e
}

func (te *TestEditor) BP() *buffer.Buffer {
	return buffer.All.Current
}

func (te *TestEditor) WP() *window.Window {
	return window.Active.CurrentWindow
}

// LoadText replaces buffer content and parks the cursor at end-of-buffer.
func (te *TestEditor) LoadText(text string) {
	te.t.Helper()
	bp := te.BP()
	wp := te.WP()
	if bp == nil || wp == nil {
		te.t.Fatal("no buffer/window")
	}
	bp.IsChanged = false
	bp.Clear()
	for line := range strings.SplitSeq(text, "\n") {
		bp.AppendLineBytes([]byte(line))
	}
	if bp.LineCount > 0 {
		wp.Cursor.Line = bp.LineCount
		last := bp.Line(bp.LineCount)
		if last != nil {
			wp.Cursor.Offset = last.Len()
		} else {
			wp.Cursor.Offset = 0
		}
	} else {
		wp.Cursor = buffer.Location{Line: bp.EOF(), Offset: 0}
	}
	wp.Mark = wp.Cursor
	bp.IsChanged = false
}

// BufferText returns buffer lines joined with newlines (C buffer_to_string).
func (te *TestEditor) BufferText() string {
	bp := te.BP()
	if bp == nil {
		return ""
	}
	lines := make([]string, 0, int(bp.LineCount))
	for i := uint(1); i <= bp.LineCount; i++ {
		lp := bp.Line(i)
		if lp == nil {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, string(lp.Data))
	}
	return strings.Join(lines, "\n")
}

func (te *TestEditor) SetCursor(line, offset uint) {
	te.t.Helper()
	wp := te.WP()
	if wp == nil {
		te.t.Fatal("no window")
	}
	wp.Cursor = buffer.Location{Line: line, Offset: offset}
}

func (te *TestEditor) Cursor() buffer.Location {
	wp := te.WP()
	if wp == nil {
		return buffer.Location{}
	}
	return wp.Cursor
}

func (te *TestEditor) SetLangMode(lang buffer.LangMode) {
	bp := te.BP()
	if bp == nil {
		te.t.Fatal("no buffer")
	}
	bp.LangMode = lang
	mode.ApplyLangIndentDefaults(bp)
}

// Key dispatches one editor key through the event Handle path.
func (te *TestEditor) Key(k uint32) bool {
	return Handle(State, event.KeyEvent{Code: k})
}

// Click sets screen mouse coordinates and dispatches a left-click command.
func (te *TestEditor) Click(row, col uint32) {
	te.t.Helper()
	display.Active.Mouse.Row = row
	display.Active.Mouse.Col = col
	if !Execute(int(term.MouseLeft), false, 1) {
		te.t.Fatalf("mouse left click at (%d,%d) failed", row, col)
	}
}

// Keys types literal text and special keys (\n -> Enter).
func (te *TestEditor) Keys(s string) {
	te.t.Helper()
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\n' {
			te.Key(term.KeyEnter)
			continue
		}
		if !te.Key(uint32(c)) {
			te.t.Fatalf("insert %q failed", string(c))
		}
	}
}

// Press dispatches human-readable key chords (e.g. "C-d", "C-a", "RET", "C-x C-s").
func (te *TestEditor) Press(chords ...string) {
	te.t.Helper()
	for _, chord := range chords {
		k, ok := parseKeySequence(strings.TrimSpace(chord))
		if !ok {
			te.t.Fatalf("invalid key chord %q", chord)
		}
		if !Execute(int(k), false, 1) {
			te.t.Fatalf("key %q dispatch failed", chord)
		}
	}
}

// Cmd runs a command with undo wrapping like interactive dispatch.
func (te *TestEditor) Cmd(fn CommandFunc, f bool, n int) bool {
	BeginCommand()
	defer EndCommand()
	return fn(f, n)
}

func (te *TestEditor) Undo() {
	te.t.Helper()
	if !CmdUndo(false, 1) {
		te.t.Fatal("undo failed")
	}
}

func (te *TestEditor) ForgetUndo() {
	bp := te.BP()
	if bp != nil {
		ForgetBuffer(bp)
	}
}

func (te *TestEditor) Edit(begin, end buffer.Location, text string) {
	te.t.Helper()
	bp := te.BP()
	if bp == nil {
		te.t.Fatal("no buffer")
	}
	BeginCommand()
	defer EndCommand()
	data := []byte(text)
	if !bufferSetText(bp, begin, end, data, nil, false) {
		te.t.Fatalf("bufferSetText(%q) failed", text)
	}
}

func (te *TestEditor) ExpectText(want string) {
	te.t.Helper()
	if got := te.BufferText(); got != want {
		te.t.Fatalf("buffer text = %q, want %q", got, want)
	}
}

func (te *TestEditor) ExpectLineCount(want uint) {
	te.t.Helper()
	if got := te.BP().LineCount; got != want {
		te.t.Fatalf("line_count = %d, want %d", got, want)
	}
}

func (te *TestEditor) ExpectCursor(line, offset uint) {
	te.t.Helper()
	cur := te.Cursor()
	if cur.Line != line || cur.Offset != offset {
		te.t.Fatalf("cursor = (%d,%d), want (%d,%d)", cur.Line, cur.Offset, line, offset)
	}
}

func (te *TestEditor) ExpectChanged(want bool) {
	te.t.Helper()
	if te.BP().IsChanged != want {
		te.t.Fatalf("is_changed = %v, want %v", te.BP().IsChanged, want)
	}
}

// NewlineIndent presses Enter once with undo grouping and returns the new column.
func (te *TestEditor) NewlineIndent() uint {
	te.t.Helper()
	BeginCommand()
	defer EndCommand()
	if !CmdModeNewlineAndIndent(false, 1) {
		te.t.Fatal("newline-and-indent failed")
	}
	return te.Cursor().Offset
}
