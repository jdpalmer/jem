# jem Quick Reference

**jem** is a lightweight terminal text editor in the spirit of MicroEMACS.
It supports multiple buffers, split windows, syntax highlighting, language-aware
indentation, incremental search, and query-replace.

---

## 1. Starting jem

```
jem [file ...]
```

Multiple files may be given on the command line.  Each is loaded into its own
buffer; the first file is displayed on startup.

---

## 2. Notation

| Notation | Meaning |
|----------|---------|
| `C-x` | Hold Ctrl and press `x` |
| `M-x` | Hold Alt (Meta) and press `x`, or press Esc then `x` |
| `C-x C-f` | Ctrl+X then Ctrl+F (two-keystroke sequence) |

---

## 3. Files

| Key | Action |
|-----|--------|
| `C-x C-f` | Visit (open) a file |
| `C-x C-r` | Read a file into the current buffer (replacing its contents) |
| `C-x C-s` | Save the current buffer to its file |
| `C-x C-w` | Write the current buffer to a new file name |
| `C-x C-v` | Revert the current buffer to the on-disk file |

The file prompt supports **Tab completion** of file and directory names.

Files modified on disk since they were loaded are automatically reloaded if
the buffer has no unsaved changes.  If the buffer has unsaved changes, jem
prompts before reverting (unless `auto-revert-mode` is set to `1` in
`~/.jem.json`).

To change line-ending mode (LF / CRLF / CR), use `M-x set_eol_mode`.

---

## 4. Cursor Motion

| Key | Action |
|-----|--------|
| `→` | Forward one character |
| `←` | Backward one character |
| `↓` | Forward one line |
| `↑` | Backward one line |
| `M-f` / `Shift-→` | Forward one word |
| `M-b` / `Shift-←` | Backward one word |
| `C-a` | Beginning of line |
| `C-e` | End of line |
| `M-m` | Back to indentation (first non-whitespace on line) |
| `C-v` / `Shift-↓` | Forward one page |
| `M-v` / `Shift-↑` | Backward one page |
| `M-<` | Beginning of buffer |
| `M->` | End of buffer |
| `M-g` | Go to line number |
| `C-M-f` / `C-M-b` | Forward / backward sexp |
| `C-x =` | Show current position (line, column, character) |

Emacs `C-f` / `C-b` / `C-n` / `C-p` are not bound in the main editor by
default; use the arrow keys, or add bindings under `keybindings` in
`~/.jem.json` (see section 16).

---

## 5. Editing

| Key | Action |
|-----|--------|
| `C-d` | Delete character forward |
| `DEL` / `Backspace` | Delete character backward |
| `M-d` | Delete word forward |
| `M-h` / `M-DEL` | Delete word backward |
| `C-k` | Kill (cut) to end of line; repeat to kill multiple lines |
| `C-o` | Open (insert) a blank line |
| `C-t` | Transpose characters |
| `M-t` | Transpose words |
| `C-x C-t` | Transpose the current line with the line above |
| `M-u` | Uppercase word |
| `M-l` | Lowercase word |
| `M-c` | Capitalize word |
| `M-\` | Trim whitespace around cursor |
| `M-q` | Fill paragraph to `fill-column` |
| `C-x C-o` | Delete blank lines around cursor |
| `C-q` | Quoted insert: insert the next key literally |
| `C-z` | Undo the most recent editing command |
| `Enter` | Insert newline (with smart indent in language modes) |
| `Tab` | Re-indent current line (in language modes); insert tab otherwise |

---

## 6. Kill Ring (Cut and Paste)

| Key | Action |
|-----|--------|
| `C-k` | Kill to end of line (accumulates with repeated kills) |
| `C-w` | Kill (cut) the selected region |
| `M-w` | Copy the selected region |
| `C-y` | Yank (paste) the most recently killed text |

Consecutive `C-k` commands accumulate into a single entry so that the entire
block can be yanked back in one `C-y`.

`C-w` and `M-w` copy to the system clipboard when available.  `C-y` prefers
the clipboard and falls back to the kill ring.  Bracketed paste from the
terminal is supported.

---

## 7. Mark and Region

| Key | Action |
|-----|--------|
| `C-Space` | Set the mark at the cursor |
| `C-x C-x` | Swap cursor and mark |
| `C-x SPC` | Push the current location onto the mark stack |
| `C-x C-SPC` | Pop back to the most recently pushed mark |
| `C-x h` | Mark the entire buffer |
| `C-x C-u` | Uppercase the region |
| `C-x C-l` | Lowercase the region |

The region is the text between the cursor and the mark.  It is highlighted in
a distinctive color.

---

## 8. Search and Replace

| Key | Action |
|-----|--------|
| `C-s` | Incremental search forward |
| `C-r` | Incremental search backward |
| `M-C-s` | Incremental regex search forward |
| `M-C-r` | Incremental regex search backward |
| `M-%` | Query replace |
| `M-.` | Jump to the tag definition at point |

Use `M-x toggle_search_scope` or `M-x set_variable` → `search-scope` to
switch search and replace between the current buffer and all open buffers.

Use `M-x query_re_replace` for query replace with a regular expression.

### Incremental Search (`C-s` / `C-r`)

Type characters to extend the search pattern; the cursor jumps to each match
as you type.

| Key in search | Action |
|---------------|--------|
| printable char | Extend pattern and jump to next match |
| `C-s` | Jump to next match |
| `C-r` | Jump to previous match |
| `DEL` / `Backspace` | Remove last character from pattern |
| `Enter` | Accept and stay at current match |
| `C-g` / `Esc` | Cancel and return to starting position |
| any other key | Accept current match and execute that key |

### Query Replace (`M-%`)

Prompts for a search pattern, then a replacement string.  At each match:

| Key | Action |
|-----|--------|
| `y` or `Space` | Replace and move to next match |
| `n` or `Del` | Skip this match |
| `!` | Replace all remaining matches |
| `q` or `Enter` | Stop replacing |
| `?` | Show help |

Tag navigation uses `tags.json` generated by `make tags`.  Override the tags
file with the `JEM_TAGS_FILE` environment variable.  When the cursor is inside
a function call and ctags provides a signature, jem shows a parameter hint in
the message line.

---

## 9. Command Palette and Menu

| Key | Action |
|-----|--------|
| `M-x` | Command palette (fuzzy search all commands) |
| `C-/` or `C-_` | Message-line menu (Open, Save, Undo, Yank, Search, Quit) |

Most commands without a default keybinding can be run from the command palette.

---

## 10. Buffers

| Key | Action |
|-----|--------|
| `C-x b` | Switch to a buffer (interactive picker) |
| `C-x k` | Kill (close) a buffer (interactive picker, defaults to current) |

The interactive buffer picker shows all open buffers.  Use `C-f` / `C-b` or
the arrow keys to navigate, `Enter` to select, `C-g` / `Esc` to cancel.

---

## 11. Windows

| Key | Action |
|-----|--------|
| `C-x 2` | Split the window horizontally |
| `C-x 1` | Expand the current window to fill the screen |
| `C-x 0` | Delete the current window |
| `C-x o` | Move to the other window |

---

## 12. Macros

| Key | Action |
|-----|--------|
| `C-x (` | Start recording a keyboard macro |
| `C-x )` | Stop recording |
| `C-x e` | Execute the last recorded macro |

---

## 13. Miscellaneous

| Key | Action |
|-----|--------|
| `C-l` | Redraw the screen |
| `C-u` | Universal argument (default: 4); prefix a command to repeat it |
| `C-g` / `Esc` | Abort current operation / cancel prompt |
| `C-x !` | Run a shell command; output goes to a new buffer |
| `M-!` | Open an interactive shell in a new buffer |
| `C-x d` | Insert the current date |
| `C-x t` | Toggle dark / light colour theme |
| `Shift-Tab` | Request identifier completion at point |
| `Shift-Enter` | Accept the pending completion |
| `C-x C-c` | Quit jem (prompts if there are unsaved changes) |

---

## 14. Project Tools

These commands have no default keybinding; invoke them with `M-x`:

| Command | Action |
|---------|--------|
| `grep_project` | Ripgrep project search → `*grep*` buffer; `Enter` jumps to match |
| `compile` | Run a build command → `*compile*` buffer; `Enter` visits diagnostic |
| `describe_command` | Show one command name and its description |
| `describe_variable` | Show one variable value and description |
| `set_variable` | Interactively set an editor variable |
| `copy_register` | Copy the active region to a named register |
| `insert_register` | Insert the contents of a named register |
| `sort_region` | Sort the active region by lines |

---

## 15. Language Modes

Language modes are activated automatically based on file extension (see table
below).  They provide smart indentation and navigation commands.  Additional
modes are supported internally but are not auto-detected from the file name.

### Auto-Detected Languages

| Extension(s) | Mode |
|--------------|------|
| `.go` | Go |
| `.py` | Python |
| `.c`, `.h` | C |
| `.java` | Java |
| `.md`, `.markdown` | Markdown |
| `.html`, `.htm` | HTML/XML |
| `.css` | CSS |
| `.js` | JavaScript |
| `.ts` | TypeScript |
| `.rs` | Rust |
| *(other)* | Text |

### Language Commands (all modes)

| Key | Action |
|-----|--------|
| `Enter` | Insert newline and auto-indent |
| `Tab` | Re-indent the current line |
| `C-\` | Jump to matching bracket / paren / brace |
| `M-;` | Add or jump to a comment on the current line |
| `M-C-a` | Go to the top of the current function / class |
| `M-C-e` | Go to the end of the current function / class |
| `M-C-h` | Mark (select) the current function / class |
| `}` | Insert `}` and auto-indent *(C-family modes: C, Java, Go, Rust, Swift, JS/TS, Kotlin, Dart, C#, etc.)* |

### C-Family Indentation Parameters

Set in `~/.jem.json` (see section 17):

| Variable | Default | Meaning |
|----------|---------|---------|
| `c-indent` | `2` | Spaces added after `{` |
| `c-brace` | `0` | Extra indent for a standalone `{` line |
| `c-colon-offset` | `0` | Offset of `case`/`default` labels inside `switch` |

### Python Indentation Parameters

| Variable | Default | Meaning |
|----------|---------|---------|
| `py-indent` | `4` | Spaces per block level |
| `py-continued-offset` | `4` | Extra indent after a `\` continuation |

Python's `elif`, `else`, `except`, and `finally` keywords are automatically
de-indented to align with their opening block.

---

## 16. Syntax Highlighting

Syntax highlighting is applied automatically for all supported language modes.
Brackets, parentheses, and braces are highlighted with **rainbow colours** that
cycle independently for each bracket type:

- `(` … `)` — starting from purple
- `[` … `]` — starting from blue
- `{` … `}` — starting from green

---

## 17. Colour Themes

jem ships with Solarized-based dark and light themes.

| Method | Action |
|--------|--------|
| `C-x t` | Toggle between dark and light theme |
| `"theme-mode": 1` in `~/.jem.json` | Start in light mode |

---

## 18. Configuration

jem reads configuration from `~/.jem.json` at startup.  Variable names use
hyphenated keys:

```json
{
  "fill-column": 100,
  "theme-mode": 1,
  "startup-quote": 0,
  "whitespace-cleanup": 1,
  "search-scope": 0,
  "auto-revert-mode": 0,
  "c-indent": 4,
  "c-brace": 0,
  "c-colon-offset": 0,
  "py-indent": 2,
  "py-continued-offset": 4,
  "keybindings": {
    "C-x C-e": "set_eol_mode",
    "C-x f": "set_variable",
    "C-f": "forward_char",
    "C-b": "backward_char",
    "C-n": "forward_line",
    "C-p": "backward_line"
  }
}
```

| Variable | Default | Meaning |
|----------|---------|---------|
| `fill-column` | `80` | Column width used by paragraph fill (`M-q`) |
| `theme-mode` | `0` | Set to `1` to start in light (Solarized light) theme |
| `startup-quote` | `1` | Set to `0` to suppress the startup quote |
| `whitespace-cleanup` | `1` | Set to `0` to preserve trailing whitespace when saving |
| `search-scope` | `0` | `0` = current buffer, `1` = all buffers for search/replace |
| `auto-revert-mode` | `0` | `0` = prompt before reverting modified buffers on disk change; `1` = always reload |
| `c-indent` | `2` | C-family: spaces per block level |
| `c-brace` | `0` | C-family: extra indent for standalone `{` |
| `c-colon-offset` | `0` | C-family: offset of `case`/`default` labels |
| `py-indent` | `4` | Python: spaces per block level |
| `py-continued-offset` | `4` | Python: extra indent after `\` continuation |

### Custom Keybindings

The `keybindings` object maps key chords to command names (as shown in `M-x`).
Chords use Emacs-style notation (`C-`, `M-`, `S-`, and `C-x` prefixes).
Set a value to `null` to unbind a key.  See `editor/config.go` for parsing
rules.

### Environment Variables

Only a few environment variables are read directly:

| Variable | Meaning |
|----------|---------|
| `JEM_TAGS_FILE` | Path to the tags file (default: `tags.json`, searched upward from the buffer) |
| `JEM_CPU_PROFILE` | Write a CPU profile to this path at startup |
| `JEM_CPU_PROFILE_SECONDS` | Optional duration for the startup CPU profile |

---

## 19. Mouse Support

| Action | Effect |
|--------|--------|
| Left click | Move cursor to clicked position |
| Click and drag | Move cursor; extend selection if mark is set |
| Scroll wheel up | Scroll up |
| Scroll wheel down | Scroll down |

---

## 20. The Mode Line

Each window has a mode line at the bottom, roughly:

```
*D m s b  jem <ver> | <buffer> | <EOL> | <Lang> | [git] | <pct>% L<n> C<n>     Ctrl+/ = Menu
```

Status indicators (left to right):

| Char | Meaning |
|------|---------|
| `*` | Buffer has unsaved changes (space if clean) |
| `D` | File changed on disk while buffer is modified and local edits were kept |
| `m` | Keyboard macro is being recorded (shown in red when active) |
| `s` | Mark stack is non-empty |
| `b` | Search-scope indicator slot |

Fields after the indicators:

- **jem version** and buffer name
- **EOL** mode (`LF`, `CRLF`, or `CR`)
- **Language** mode name (e.g. `C`, `Go`, `Python`)
- **Git** branch name (when inside a git repository)
- **Position**: percentage, line number, and column (`L<n> C<n>`)
- **`^`** at column 80 marks the fill column
- **`Ctrl+/ = Menu`** hint on the right

---

## 21. Quick Tips

- **Multiple files:** pass several file names on the command line; switch
  between them with `C-x b`.
- **Paragraph fill:** set `fill-column` in `~/.jem.json` or via
  `M-x set_variable`, then use `M-q` to fill the current paragraph.
- **Repeat a command:** prefix with `C-u` (default ×4) or `C-u` followed by a
  number (e.g. `C-u 8 C-k` kills 8 lines).
- **Undo:** use `C-z` to undo the most recent editing command, or pick Undo
  from the menu (`C-/`).
- **Emacs motion keys:** bind `C-f` / `C-b` / `C-n` / `C-p` in the
  `keybindings` section of `~/.jem.json` if you want them in the main editor.
