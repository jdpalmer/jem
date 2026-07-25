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

## Features

* Windows, macOS, and Linux support
* Easy installation: a single portable executable
* Emacs-style editing commands and `M-x` command palette
* Mouse support (click, drag, scroll wheel)
* Multiple buffers and split windows
* UTF-8 files; LF, CRLF, and CR line endings
* Fast DFA-based syntax highlighting and language-aware indentation
* Incremental and regex search; query-replace
* Tags-based navigation and call-site signature hints
* Project grep and compile/diagnostic buffers
* Git gutter markers and modeline branch display
* Fuzzy matching for files, commands, buffers, and more
* System clipboard integration
* `~/.jem.json` configuration with custom keybindings

## Anti-Features

* No GUI support; modern terminals are GPU accelerated and have advanced key
  events and 24-bit color support
* No legacy platform support; jem uses modern terminal features extensively
* No extensibility language; compiling jem is fast; and extending it in
  Go is easy
* No support for anything except [UTF-8](https://utf8everywhere.org/)

## Motivation

jem (short for James's Emacs) began as a C fork of Dave Conroy's (1985)
bare bones public domain `uemacs`. I ported my private fork to Go and have
continued adding features influenced by GNU Emacs, John E. Davis's JED, Linus
Torvalds's uemacs fork, and others.

## License

MIT — see [LICENSE](LICENSE).
