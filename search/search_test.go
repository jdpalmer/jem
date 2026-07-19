package search

import (
	"testing"

	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/buffer"
)

func makeTestWindow(t *testing.T, text string) *model.Window {
	t.Helper()
	model.Reset()
	DefaultState = &State{}
	bp := model.BufferCreate(&model.State.EditorRuntimeState)
	if bp == nil {
		t.Fatal("buffer create failed")
	}
	bp.Name = "*test*"
	wp := model.WindowCreate()
	if wp == nil {
		t.Fatal("window create failed")
	}
	wp.Buffer = bp
	model.WindowSelect(wp)
	eof := buffer.MakeLocation(bp.EOF(), 0)
	data := []byte(text)
	if err := bp.SetText(nil, buffer.MakeLocation(1, 0), eof, data, nil); err != nil {
		t.Fatal("buffer.SetText failed")
	}
	return wp
}

func TestFindNextPlain(t *testing.T) {
	wp := makeTestWindow(t, "hello world\nfoo bar\n")
	wp.SetCursor(buffer.Location{Line: 1, Offset: 0})
	DefaultState.SearchCaseSensitive = true
	if !findNextPlain(wp, []byte("world")) {
		t.Fatal("expected to find world")
	}
	if wp.Cursor.Line != 1 || wp.Cursor.Offset != 11 {
		t.Fatalf("cursor at %+v, want line 1 offset 11", wp.Cursor)
	}
	if !findNextPlain(wp, []byte("foo")) {
		t.Fatal("expected to find foo")
	}
	if wp.Cursor.Line != 2 || wp.Cursor.Offset != 3 {
		t.Fatalf("cursor at %+v, want line 2 offset 3", wp.Cursor)
	}
}

func TestFindPrevPlain(t *testing.T) {
	wp := makeTestWindow(t, "abc abc\n")
	wp.SetCursor(buffer.Location{Line: 1, Offset: 7})
	DefaultState.SearchCaseSensitive = true
	if !findPrevPlain(wp, []byte("abc")) {
		t.Fatal("expected to find abc")
	}
	if wp.Cursor.Line != 1 || wp.Cursor.Offset != 4 {
		t.Fatalf("cursor at %+v, want line 1 offset 4", wp.Cursor)
	}
}

func TestRegexMatchForward(t *testing.T) {
	wp := makeTestWindow(t, "foo123bar\n")
	match, found := findNextRegexMatchFrom(wp.Buffer, buffer.Location{Line: 1, Offset: 0}, `[0-9]+`)
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
