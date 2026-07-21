package syntax

import (
	"bytes"
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestFindMatchingDelimiterSingleLine(t *testing.T) {
	buf := makeBufferFromLines([]string{"{ }"})
	var match buffer.Location
	if !FindMatchingDelimiter(buf, buffer.MakeLocation(1, 0), &match) {
		t.Fatal("expected match on single-line buffer")
	}
	if match.Line != 1 || match.Offset != 2 {
		t.Fatalf("expected (1,2) got (%d,%d)", match.Line, match.Offset)
	}
}

func TestFindMatchingDelimiterLastLine(t *testing.T) {
	buf := makeBufferFromLines([]string{
		"package main",
		"func f() {",
		"}",
	})
	line2 := buf.Line(2)
	openOff := bytes.IndexByte(line2.Data, '{')
	var match buffer.Location
	if !FindMatchingDelimiter(buf, buffer.MakeLocation(2, openOff), &match) {
		t.Fatal("expected forward match to last line")
	}
	if match.Line != 3 || match.Offset != 0 {
		t.Fatalf("expected close at (3,0) got (%d,%d)", match.Line, match.Offset)
	}
	if !FindMatchingDelimiter(buf, buffer.MakeLocation(3, 0), &match) {
		t.Fatal("expected backward match from last line")
	}
	if match.Line != 2 || match.Offset != openOff {
		t.Fatalf("expected open at (2,%d) got (%d,%d)", openOff, match.Line, match.Offset)
	}
}

func TestCharIsStructuralStringOnLastLine(t *testing.T) {
	buf := makeBufferFromLines([]string{`fmt.Println("{")`})
	line := buf.Line(1)
	braceOff := bytes.IndexByte(line.Data, '{')
	if CharIsStructural(buf, 1, braceOff) {
		t.Fatal("brace inside string on last line should not be structural")
	}
}

func TestFindMatchingDelimiterMultibyte(t *testing.T) {
	buf := makeBufferFromLines([]string{"日[日]"})
	line := buf.Line(1)
	openOff := bytes.IndexByte(line.Data, '[')
	closeOff := bytes.IndexByte(line.Data, ']')
	var match buffer.Location
	if !FindMatchingDelimiter(buf, buffer.MakeLocation(1, openOff), &match) {
		t.Fatal("expected multibyte line to match brackets")
	}
	if match.Offset != closeOff {
		t.Fatalf("expected close at byte %d got %d", closeOff, match.Offset)
	}
}

func TestByteOffsetToRuneLimit(t *testing.T) {
	line := makeLine("日[")
	tests := []struct {
		byteOff int
		want    int
	}{
		{0, 0},
		{1, 0}, // mid-first-rune
		{3, 1}, // after 日
		{4, 2}, // after [
	}
	for _, tc := range tests {
		if got := byteOffsetToRuneLimit(line, tc.byteOff); got != tc.want {
			t.Fatalf("byteOffsetToRuneLimit(%d) = %d, want %d", tc.byteOff, got, tc.want)
		}
	}
}
