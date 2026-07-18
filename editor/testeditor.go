package editor

import (
	"strings"
	"testing"

	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/tools"
)

// TestEditor is a headless editor instance for integration tests.
type TestEditor struct {
	t *testing.T
}

// NewTestEditor resets package globals and returns a fresh headless editor.
func NewTestEditor(t *testing.T) *TestEditor {
	t.Helper()
	resetTestEditorState()
	VarsInit()
	DisplayInitHeadless(24, 80)
	KeybindingsInit()
	backgroundJobsInit()
	EditorInit("test")
	if app.State.CurrentWindow != nil {
		app.State.CurrentWindow.Height = uint32(term.Rows())
	}
	return &TestEditor{t: t}
}

func resetTestEditorState() {
	app.State = App{}
	editorUndo = UndoHistory{}
	killRing = [16][]byte{}
	killRingCount = 0
	killRingIdx = 0
	killAggregate = nil
	registerStore = make(map[string][]byte)
	completionPending = ""
	quitRequested = false
	tools.ResetBackgroundJobsForTests()
}

func (te *TestEditor) BP() *Buffer {
	return app.State.CurrentBuffer
}

func (te *TestEditor) WP() *Window {
	return app.State.CurrentWindow
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
		bp.AppendLineBytes([]byte(line), uint(len(line)))
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
		wp.Cursor = Location{Line: bp.EOF(), Offset: 0}
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
	wp.Cursor = Location{Line: line, Offset: offset}
}

func (te *TestEditor) Cursor() Location {
	wp := te.WP()
	if wp == nil {
		return Location{}
	}
	return wp.Cursor
}

func (te *TestEditor) SetLangMode(mode LangMode) {
	bp := te.BP()
	if bp == nil {
		te.t.Fatal("no buffer")
	}
	bp.LangMode = mode
}

// Key dispatches one editor key through the normal command path.
func (te *TestEditor) Key(k uint32) bool {
	return Execute(int(k), false, 1)
}

// Click sets screen mouse coordinates and dispatches a left-click command.
func (te *TestEditor) Click(row, col uint32) {
	te.t.Helper()
	app.State.Mouse.Row = row
	app.State.Mouse.Col = col
	if !Execute(int(MouseLeft), false, 1) {
		te.t.Fatalf("mouse left click at (%d,%d) failed", row, col)
	}
}

// Keys types literal text and special keys (\n -> Enter).
func (te *TestEditor) Keys(s string) {
	te.t.Helper()
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\n' {
			te.Key(KeyEnter)
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
	UndoBeginCommand()
	defer UndoEndCommand()
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
		UndoForgetBuffer(bp)
	}
}

func (te *TestEditor) Edit(begin, end Location, text string) {
	te.t.Helper()
	bp := te.BP()
	if bp == nil {
		te.t.Fatal("no buffer")
	}
	UndoBeginCommand()
	defer UndoEndCommand()
	data := []byte(text)
	if !bufferSetText(bp, begin, end, data, uint(len(data)), nil, false) {
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
	UndoBeginCommand()
	defer UndoEndCommand()
	if !CmdModeNewlineAndIndent(false, 1) {
		te.t.Fatal("newline-and-indent failed")
	}
	return te.Cursor().Offset
}
