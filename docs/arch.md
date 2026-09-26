# jem architecture

This document describes the `%C` port in `jem.mc/` (feature reference was the
Go tree in `jem.go/`).

## Data flow

```text
Events → Handlers → Buffer / Window / … → Display
```

Dependencies run in the **reverse** direction of that pipeline:

- `display` and handlers depend on `buffer`, `window`, `minibuffer` (not `runtime`)
- `event` imports almost nothing
- `event` owns the ring (`event_enqueue` / `event_try_dequeue`); producers must
  not import `runtime` to post
- `runtime` owns the loop: dequeue from the event ring → handle (listener stack,
  then window dispatch)

**Events** may be produced by the key reader, handlers, or async workers, but are
always **queued** and applied on a later tick. Handle must not call itself
synchronously for follow-ups—enqueue instead.

## Packages

| Package | Role |
|---------|------|
| `event` | Input events + ring |
| `buffer` | Document, buffer list, undo history, UTF-8 offset helpers, display-column ↔ byte-offset on lines |
| `window` | Window list, layout, match window, cursor point and goal column |
| `killring` / `markring` / `minibuffer` / `register` | Kill ring, mark ring, minibuffer edit state, named registers |
| `clipboard` | Native clipboard helpers + OSC 52 write fallback |
| `file` | Path helpers + load/save/reload |
| `display` | Redisplay, git gutter/modeline |
| `runtime` | Command loop, listeners, commands, config/macros/vars |
| `mode` / `syntax` / `term` / `search` / `regex` | Domain helpers |
| `job` | Async grep/compile/spawn; posts completion via the event ring |
| `tags` | Project tags load/build, navigation, completion, describe |
| `json` | DOM JSON for `~/.jem.json` |

## Undo and edits

`buffer` owns interactive undo (`begin_command` / `end_command`, `set_text`).
Interactive shell edits go through window text APIs (adjust cursors, dirty flags,
syntax reparse). Raw buffer replace helpers are text-only.

## Config sync

`vars_init` / variable writes push theme palette into syntax and create defaults
into buffer. Leaves call peers directly or return values for runtime commands to
apply.

**Commands own UI.** Ask* lives only in `runtime`. `job` / `tags` / `search`
expose domain entry points; runtime commands prompt then call them (and push
key sessions onto the listener stack).

## Listener stack

Modal prompts push a listener onto a stack inside `runtime`. The outer handle
gives the top listener priority. When the prompt finishes, the listener pops.

Prompts use async ask-yes-no / ask-string / ask-filename / ask-fuzzy / ask-choose
(push listener + continuation) on the `runtime` path.

Bracketed paste is applied via paste events on the handler path (not inside
display update). Mouse, refresh, and theme-toggle commands live in `runtime`;
display only hit-tests screen geometry and renders. The `*match*` window is
owned by `window`. Minibuffer text editing and history live in `minibuffer`;
display renders prompts and dispatches keys.

Multi-key modes (isearch, query-replace confirm) use a search key session pushed
via the same stack—one key per tick.

`C-u` is a universal-argument listener on the stack; the terminating key runs through
bound-key dispatch. Named commands can also be applied via command events.
Keyboard macros store event sequences; ask prompts consume replies during
playback. The key reader always enqueues key events—there is no separate
minibuf key channel.

## Import rules

- `buffer` must not import `window`, `display`, `runtime`, `markring`, or `minibuffer`
- `window` may import `buffer`; must not import `display` or `runtime`
- Cursor moves and goal-column updates go through `window` APIs. Do not assign
  `win.cursor` directly outside `window`.
- `killring` / `register` import almost nothing (clipboard is a peer of killring)
- `file` may import `buffer` / `mode`; must not import `runtime` or `display`.
  Process flags are passed into reload checks by runtime.
- `display` must not import `runtime`
- `event` must not import `runtime`, `display`, or the session packages above
- `json` must not import editor packages
