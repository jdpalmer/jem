# jem

jem is a terminal text editor in the spirit of MicroEMACS, written in Go.

## Build

Requires Go 1.25 or later.

```bash
go build -o jem .
./jem [files...]
```

Install to `$GOPATH/bin` or `$GOBIN`:

```bash
go install .
```

## Documentation

- [Quick reference](docs/QUICKREF.md)

## Configuration

jem reads `~/.jem.json` at startup (theme, indentation, search scope, custom
keybindings). See the [quick reference](docs/QUICKREF.md#18-configuration) for
all options.

## Features

* Windows, macOS, and Linux support (CI: Linux and macOS)
* Single portable executable
* Emacs-style editing commands and `M-x` command palette
* Light and dark Solarized themes
* Mouse support (click, drag, scroll wheel)
* Multiple buffers and split windows
* UTF-8 files; LF, CRLF, and CR line endings
* Undo
* Fast DFA-based syntax highlighting and language-aware indentation
* Incremental and regex search; query-replace
* Tags-based navigation and call-site signature hints
* Native identifier completion
* Project grep (`ripgrep`) and compile/diagnostic buffers
* Git gutter markers and branch display in the modeline
* System clipboard integration
* `~/.jem.json` configuration with custom keybindings

## Anti-Features

* No GUI support; modern terminals are GPU accelerated and have advanced key
  events and 24-bit color support
* No legacy platform support; jem uses modern terminal features extensively and
  is not encumbered by an architecture designed to support terminal standards
  that date back to the 1970s
* No extensibility language; compiling jem is fast; and extending it in
  Go is easy

## Motivation

jem (short for James's Emacs) began as a C fork of Dave Conroy's (1985)
bare bones public domain `uemacs`. I ported my fork to Go and have continued
adding features influenced by GNU Emacs, John E. Davis's JED, Linus Torvalds's
uemacs fork, and others.

## License

MIT — see [LICENSE](LICENSE).
