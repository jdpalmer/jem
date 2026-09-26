# jem

jem is a terminal text editor in the spirit of MicroEMACS, written in
[%C](../modc). This tree is a port of the Go implementation in
[`../jem.go`](../jem.go).

## Features

* macOS and Linux support (Windows is deferred)
* Easy installation: a single portable executable
* Emacs-style editing commands and `M-x` command palette
* Mouse support (click, drag, scroll wheel)
* Multiple buffers and split windows
* UTF-8 files; LF and CRLF line endings (bare CR is converted to LF on load)
* Fast DFA-based syntax highlighting and language-aware indentation
* Incremental and regex search; query-replace
* Tags-based navigation and call-site signature hints
* Project grep and compile/diagnostic buffers
* Git gutter markers and modeline branch display
* Fuzzy matching for files, commands, buffers, and more
* System clipboard integration
* `~/.jem.json` configuration with custom keybindings

## Anti-Features

* No GUI support; leverages GPU acceleration, 24-bit color, and advanced key
  events available in modern terminals like Kitty, WezTerm, Ghostty, foot, and
  Windows Terminal
* No legacy platform support; jem uses modern terminal features extensively
  (Kitty keyboard protocol required)
* No extensibility language; compiling jem is fast; and extending it in
  %C is easy
* [UTF-8](https://utf8everywhere.org/) only

## Build

Requires `modc` and `qbe` on `PATH`. Sibling packages resolve via `-M .`:

```bash
cd jem.mc
modc build -M . -o jem
./jem [files...]
```

## Dependencies

* [Universal Ctags](https://ctags.io/) is optional. Install it and run
  `M-x build-tags` to enable tag navigation (`M-.`), in-buffer symbol pick,
  identifier completion (`Shift-Tab` / `M-/`), call-site signature hints, and
  project-symbol details in `M-?`.
* [git](https://git-scm.com/) is optional but enables gutter change markers
  and the modeline branch name.

## Documentation

- [Quick Reference](docs/quickref.md)
- [Architecture](docs/arch.md)
- [Roadmap](planning/roadmap.md)
- [Lessons](planning/lessons.md)
- [%C dogfood log](dogfood.md)

## Motivation

jem (short for James's Emacs) began as a C fork of Dave Conroy's (1985)
bare bones public domain `uemacs`. I ported my private fork to Go and have
continued adding features influenced by GNU Emacs, John E. Davis's JED, Linus
Torvalds's uemacs fork, and others. This `%C` port dogfoods the language while
keeping Go as the feature reference.

## License

MIT — see [LICENSE](LICENSE).
