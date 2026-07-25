package display

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jdpalmer/jem/file"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/window"
)

func TestChoiceRenderIgnoresGutterClip(t *testing.T) {
	DisplayInit()
	Reset()
	buf := buffer.Create()
	if buf == nil {
		t.Fatal("buffer create failed")
	}
	buf.Name = "alpha"

	clipLeftCol = 10
	label := func(ctx any, idx int) []byte {
		buffers := ctx.([]*buffer.Buffer)
		if int(idx) >= len(buffers) || buffers[idx] == nil {
			return nil
		}
		return []byte(buffers[idx].Name)
	}
	mlChoiceRender("Buffer: ", []*buffer.Buffer{buf}, label, 1, 0, 0, 0)
	clipLeftCol = 0

	row := &backScreen.Rows[term.Rows()]
	if row.Text[0] != 'B' {
		t.Fatalf("prompt should start at column 0, got %q", row.Text[0])
	}
	want := []rune("Buffer: alpha")
	for i, c := range want {
		if row.Text[i] != c {
			t.Fatalf("col %d = %q, want %q", i, row.Text[i], c)
		}
	}
}

func TestCollectFuzzyPathsIncludesParent(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}

	paths := collectFuzzyPaths(child, "")
	if len(paths) == 0 || paths[0].Name != "../" {
		t.Fatalf("collectFuzzyPaths(child) = %v, want ../ first", paths)
	}
}

func TestCollectFuzzyPathsRootHasNoParent(t *testing.T) {
	root := string(filepath.Separator)
	paths := collectFuzzyPaths(root, "")
	for _, p := range paths {
		if p.Name == "../" {
			t.Fatalf("collectFuzzyPaths(%q) should not include ../, got %v", root, paths)
		}
	}
}

func TestEditorlyFilenameSelectionFile(t *testing.T) {
	got := file.ApplyFilenameSelection("src/", "foo.go")
	if got != "src/foo.go" {
		t.Fatalf("file.ApplyFilenameSelection = %q, want src/foo.go", got)
	}
}

func TestFilenamePromptEnterOpensDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "src")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src_test.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "main.go"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	p := NewFilenamePrompt("Find file: ", "", 0)
	p.state.SetText([]byte("src"))
	p.syncMatches()
	// Select the directory entry (ends with /), not src_test.go.
	found := -1
	for i, mi := range p.matchIndices {
		if strings.HasSuffix(p.entryName(mi), "/") {
			found = i
			break
		}
	}
	if found < 0 {
		t.Fatalf("no directory match in %#v", p.matchIndices)
	}
	p.sel = found
	done, _, _ := p.HandleKey(term.KeyEnter)
	if done {
		t.Fatal("Enter on directory should keep the prompt open")
	}
	text := string(p.state.Text)
	if !strings.HasSuffix(text, string(filepath.Separator)) {
		t.Fatalf("prompt text = %q, want trailing separator so the folder opens", text)
	}
	dirPart, pattern := file.PromptSplit(text)
	if pattern != "" {
		t.Fatalf("PromptSplit(%q) pattern = %q, want empty (list folder contents)", text, pattern)
	}
	if file.OpenDirFromPrompt(dirPart) != sub && filepath.Clean(file.OpenDirFromPrompt(dirPart)) != filepath.Clean(sub) {
		// OpenDir may be relative "src"
		if filepath.Base(strings.TrimRight(dirPart, `/\`)) != "src" {
			t.Fatalf("dirPart = %q, want src/", dirPart)
		}
	}
}

func TestEditorlyFilenameSelectionParent(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "a")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	childPrefix := child + string(filepath.Separator)
	got := file.ApplyFilenameSelection(childPrefix, "../")
	want := dir + string(filepath.Separator)
	if got != want {
		t.Fatalf("file.ApplyFilenameSelection = %q, want %q", got, want)
	}
}

func TestFilenameFuzzyScoreParentEntry(t *testing.T) {
	ok, _ := filenameFuzzyScore("../", "..")
	if !ok {
		t.Fatal("expected ../ to fuzzy-match ..")
	}
}

func TestFuzzyMatchesExceedsSixteen(t *testing.T) {
	names := make([]string, 40)
	for i := range names {
		names[i] = fmt.Sprintf("cmd%02d", i)
	}
	provider := func(ctx any, idx int) []byte {
		list := ctx.([]string)
		if idx < 0 || idx >= len(list) {
			return nil
		}
		return []byte(list[idx])
	}
	got := fuzzyMatches(provider, names, len(names), nil, fuzzyMaxMatches)
	if len(got) < 40 {
		t.Fatalf("fuzzyMatches returned %d, want all 40 (cap is %d)", len(got), fuzzyMaxMatches)
	}
}

func TestMatchListMoveSel(t *testing.T) {
	if got := matchListMoveSel(0, 20, 10); got != 10 {
		t.Fatalf("page down from 0 = %d, want 10", got)
	}
	if got := matchListMoveSel(15, 20, 10); got != 19 {
		t.Fatalf("page down near end = %d, want 19", got)
	}
	if got := matchListMoveSel(5, 20, -10); got != 0 {
		t.Fatalf("page up near start = %d, want 0", got)
	}
	if got := matchListMoveSel(0, 0, 10); got != 0 {
		t.Fatalf("empty list = %d, want 0", got)
	}
}

func TestMatchBufferNoTrailingEmptyLine(t *testing.T) {
	term.SetSize(24, 80)
	window.Active.Windows = nil
	window.Active.CurrentWindow = nil
	buffer.All.Buffers = nil
	buffer.All.Current = nil
	t.Cleanup(func() {
		window.Active.Windows = nil
		window.Active.CurrentWindow = nil
		buffer.All.Buffers = nil
		buffer.All.Current = nil
	})
	primary := window.WindowCreate()
	primary.Buffer = buffer.Create()
	window.Active.CurrentWindow = primary

	writeMatchBufferGeneric(func(out []byte, outSize int, idx int, _ any) {
		copy(out, []byte("item"))
		out[4] = 0
	}, nil, 2, 0)
	mb := buffer.Find("*match*")
	if mb == nil {
		t.Fatal("no match buffer")
	}
	if len(mb.Lines) != 2 {
		t.Fatalf("lines = %d, want 2 (no trailing empty)", len(mb.Lines))
	}
}

func TestFormatFileSize(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{-1, ""},
		{0, "0B"},
		{999, "999B"},
		{1024, "1k"},
		{4300, "4.2k"},
		{1536 * 1024, "1.5M"},
	}
	for _, tc := range cases {
		if got := formatFileSize(tc.n); got != tc.want {
			t.Fatalf("formatFileSize(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestFormatModTime(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		t    time.Time
		want string
	}{
		{time.Time{}, ""},
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-2 * time.Hour), "2h ago"},
		{now.Add(-3 * 24 * time.Hour), "3d ago"},
		{time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), "Jan 15 2024"},
	}
	for _, tc := range cases {
		if got := formatModTime(tc.t, now); got != tc.want {
			t.Fatalf("formatModTime(%v) = %q, want %q", tc.t, got, tc.want)
		}
	}
}

func TestFilenameMatchFormatterPadsColumns(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	ctx := &filenameMatchCtx{
		entries: []fuzzyFileEntry{
			{Name: "a.go", Size: 100, ModTime: now.Add(-2 * time.Hour)},
			{Name: "longer.go", Size: 1536 * 1024, ModTime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		},
		now: now,
	}
	for i := range ctx.entries {
		e := &ctx.entries[i]
		if n := len(e.Name); n > ctx.nameWidth {
			ctx.nameWidth = n
		}
		if n := len(formatFileSize(e.Size)); n > ctx.sizeWidth {
			ctx.sizeWidth = n
		}
		if n := len(formatModTime(e.ModTime, ctx.now)); n > ctx.timeWidth {
			ctx.timeWidth = n
		}
	}
	out := make([]byte, 256)
	filenameMatchFormatter(out, len(out), 0, ctx)
	end := 0
	for end < len(out) && out[end] != 0 {
		end++
	}
	got := string(out[:end])
	if !strings.HasPrefix(got, "a.go") {
		t.Fatalf("got %q, want name prefix", got)
	}
	if !strings.Contains(got, "100B") || !strings.Contains(got, "2h ago") {
		t.Fatalf("got %q, want size and relative time", got)
	}
	// Name column should pad to longer.go width.
	if got[len("a.go")] != ' ' {
		t.Fatalf("got %q, want padded name column", got)
	}
}
