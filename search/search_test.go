package search

import (
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/window"
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func makeTestWindow(t *testing.T, text string) *window.Window {
	t.Helper()
	display.Reset()
	DefaultState = &State{}
	buf := buffer.Create()
	if buf == nil {
		t.Fatal("buffer create failed")
	}
	buf.Name = "*test*"
	win := window.WindowCreate()
	if win == nil {
		t.Fatal("window create failed")
	}
	win.Buffer = buf
	window.WindowSelect(win)
	eof := buffer.MakeLocation(buf.EOF(), 0)
	data := []byte(text)
	if err := buf.SetText(nil, buffer.MakeLocation(1, 0), eof, data, nil); err != nil {
		t.Fatal("buffer.SetText failed")
	}
	return win
}

func TestFindNextPlain(t *testing.T) {
	win := makeTestWindow(t, "hello world\nfoo bar\n")
	win.SetCursor(buffer.Location{Line: 1, Offset: 0})
	DefaultState.SearchCaseSensitive = true
	if !findNextPlain(win, []byte("world")) {
		t.Fatal("expected to find world")
	}
	if win.Cursor.Line != 1 || win.Cursor.Offset != 11 {
		t.Fatalf("cursor at %+v, want line 1 offset 11", win.Cursor)
	}
	if !findNextPlain(win, []byte("foo")) {
		t.Fatal("expected to find foo")
	}
	if win.Cursor.Line != 2 || win.Cursor.Offset != 3 {
		t.Fatalf("cursor at %+v, want line 2 offset 3", win.Cursor)
	}
}

func TestFindPrevPlain(t *testing.T) {
	win := makeTestWindow(t, "abc abc\n")
	win.SetCursor(buffer.Location{Line: 1, Offset: 7})
	DefaultState.SearchCaseSensitive = true
	if !findPrevPlain(win, []byte("abc")) {
		t.Fatal("expected to find abc")
	}
	if win.Cursor.Line != 1 || win.Cursor.Offset != 4 {
		t.Fatalf("cursor at %+v, want line 1 offset 4", win.Cursor)
	}
}

func TestRegexMatchForward(t *testing.T) {
	win := makeTestWindow(t, "foo123bar\n")
	match, found := findNextRegexMatchFrom(win.Buffer, buffer.Location{Line: 1, Offset: 0}, `[0-9]+`)
	if found != 1 {
		t.Fatalf("found=%d want 1", found)
	}
	if match.Start.Offset != 3 || match.End.Offset != 6 {
		t.Fatalf("match span %+v..%+v", match.Start, match.End)
	}
}

func TestExpandRegexReplacement(t *testing.T) {
	match := RegexMatch{
		Text:  []byte("foo123bar"),
		Index: []int{3, 6, 3, 6},
		Start: buffer.Location{Line: 1, Offset: 3},
		End:   buffer.Location{Line: 1, Offset: 6},
	}
	out, err := expandRegexReplacement("\\0!", match)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "123!" {
		t.Fatalf("got %q want 123!", string(out))
	}
}

func TestUpdateSearchCase(t *testing.T) {
	updateSearchCase("hello")
	if DefaultState.SearchCaseSensitive {
		t.Fatal("expected case insensitive")
	}
	updateSearchCase("Hello")
	if !DefaultState.SearchCaseSensitive {
		t.Fatal("expected case sensitive")
	}
}
