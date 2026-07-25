package runtime

import "testing"

func TestSplitIdentParts(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"foo", []string{"foo"}},
		{"foo_bar", []string{"foo", "bar"}},
		{"FOO_BAR", []string{"foo", "bar"}},
		{"fooBar", []string{"foo", "bar"}},
		{"FooBar", []string{"foo", "bar"}},
		{"XMLHttp", []string{"xml", "http"}},
		{"foo2Bar", []string{"foo2", "bar"}},
		{"foo-bar", []string{"foo", "bar"}},
	}
	for _, tc := range cases {
		got := splitIdentParts(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("splitIdentParts(%q) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("splitIdentParts(%q) = %v, want %v", tc.in, got, tc.want)
			}
		}
	}
}

func TestConvertIdentStyles(t *testing.T) {
	cases := []struct {
		in        string
		camel     string
		pascal    string
		snake     string
		constant  string
	}{
		{"foo_bar", "fooBar", "FooBar", "foo_bar", "FOO_BAR"},
		{"FooBar", "fooBar", "FooBar", "foo_bar", "FOO_BAR"},
		{"fooBar", "fooBar", "FooBar", "foo_bar", "FOO_BAR"},
		{"FOO_BAR", "fooBar", "FooBar", "foo_bar", "FOO_BAR"},
		{"XMLHttpRequest", "xmlHttpRequest", "XmlHttpRequest", "xml_http_request", "XML_HTTP_REQUEST"},
		{"already", "already", "Already", "already", "ALREADY"},
	}
	for _, tc := range cases {
		if got := convertIdent(tc.in, identCamel); got != tc.camel {
			t.Fatalf("camel(%q) = %q, want %q", tc.in, got, tc.camel)
		}
		if got := convertIdent(tc.in, identPascal); got != tc.pascal {
			t.Fatalf("pascal(%q) = %q, want %q", tc.in, got, tc.pascal)
		}
		if got := convertIdent(tc.in, identSnake); got != tc.snake {
			t.Fatalf("snake(%q) = %q, want %q", tc.in, got, tc.snake)
		}
		if got := convertIdent(tc.in, identConstant); got != tc.constant {
			t.Fatalf("constant(%q) = %q, want %q", tc.in, got, tc.constant)
		}
	}
}

func TestCmdSnakeCaseAtPoint(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("fooBar baz")
	te.SetCursor(1, 3) // inside fooBar
	if !CmdSnakeCase(false, 1) {
		t.Fatal("CmdSnakeCase failed")
	}
	te.ExpectText("foo_bar baz")
}

func TestCmdPascalCaseAtPoint(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("foo_bar")
	te.SetCursor(1, 0)
	if !CmdPascalCase(false, 1) {
		t.Fatal("CmdPascalCase failed")
	}
	te.ExpectText("FooBar")
}
