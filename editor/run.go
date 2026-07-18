package editor

import (
	"fmt"
	"github.com/jdpalmer/jem/app"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"syscall"
	"time"

	"github.com/jdpalmer/jem/term"
	"golang.design/x/clipboard"
)

var quitRequested bool = false

// decodeKeyChar implements decode_key_char from src/main.c.
// controlContext mirrors the C parameter: when true we normalize letters for
// control-key contexts; when false we normalize Enter/Tab/Escape to special
// synthetic key codes. This takes a raw terminal-decoded key (possibly a small
// control code) and returns the editor-level key code.
func decodeKeyChar(key uint32, controlContext bool) uint32 {
	// Normalize lowercase to uppercase when in control context
	if controlContext && key >= 'a' && key <= 'z' {
		key -= 0x20
	}
	if !controlContext && key == '\t' {
		return KeyTab
	}
	if !controlContext && (key == '\r' || key == '\n') {
		return KeyEnter
	}
	if !controlContext && key == 0x1B {
		return 0x1B
	}
	if key == 0x00 {
		return CTL | ' '
	}
	if key >= 0x01 && key <= 0x1F {
		return CTL | (key + '@')
	}
	return key
}

// applyMetaPrefixToKey applies the editor-level ESC (meta) prefix to a decoded key.
// Mirrors src/main.c apply_meta_prefix_to_key behavior: if the key already has
// modifier bits, simply OR in META. For lowercase letters, convert to uppercase
// and set META so keybindings match the C implementation.
func applyMetaPrefixToKey(k uint32) uint32 {
	// If key already contains modifier bits, just add META
	if k&KeyMask != 0 {
		return k | META
	}
	// ASCII lowercase -> uppercase + META
	if k >= 'a' && k <= 'z' {
		return META | (k - ('a' - 'A'))
	}
	// Otherwise set META on the byte value
	if k < 0x100 {
		return META | k
	}
	return k | META
}

// applyCtlxPrefix forms the editor key code for the key following C-x.
// Mirrors C main.c: c = CTLX | decode_key_char(control_key, true).
func applyCtlxPrefix(second uint32) uint32 {
	second = decodeKeyChar(second, true)
	combined := CTLX | second
	// Chords like C-x b are bound as CTLX|'B'; if Ctrl stays held on the
	// second key, normalize CTLX|CTL|'B' to the plain binding when needed.
	if second&CTL != 0 {
		plain := CTLX | (second & 0xFF)
		if _, hasPlain := keybindingsMap[plain]; hasPlain {
			if _, hasCtrl := keybindingsMap[combined]; !hasCtrl {
				return plain
			}
		}
	}
	return combined
}

// Global key channels: main loop reads GlobalKeyCh; minibuffer reads GlobalMinibufKeyCh.
var GlobalKeyCh chan uint32
var GlobalMinibufKeyCh chan uint32

var editorEscapePrefixPending bool

// editorReadKey reads one editor command key on the main thread.
// Mirrors src/main.c editor_read_key (used during spawn pause while the
// background reader is frozen).
func editorReadKey(keyOut *uint32) bool {
	for {
		k, ok := term.ReadKey()
		if !ok {
			return false
		}
		k = decodeKeyChar(k, false)
		if !editorEscapePrefixPending {
			if k == 0x1B {
				editorEscapePrefixPending = true
				mbWrite("ESC")
				continue
			}
			*keyOut = k
			return true
		}
		editorEscapePrefixPending = false
		*keyOut = applyMetaPrefixToKey(k)
		return true
	}
}

func anyUnsavedBuffers() bool {
	for i := 0; i < int(app.State.BufferCount); i++ {
		bp := app.State.Buffers[i]
		if bp != nil && bp.IsChanged {
			return true
		}
	}
	return false
}

// handleEditorKey dispatches one input event. Returns false when the loop should exit.
func handleEditorKey(k uint32) bool {
	if app.State.MessagePresent {
		mbClear()
	}
	if k == 0x03 { // Ctrl-C
		if anyUnsavedBuffers() {
			if mbYesNo("Quit with unsaved buffers?") != PromptResultYes {
				return true
			}
		}
		return false
	}
	app.State.Dispatching = true
	_ = DispatchCommand(k)
	app.State.Dispatching = false
	if !app.State.MessagePresent {
		tagsMaybeShowCallHint()
	}
	if quitRequested {
		if anyUnsavedBuffers() {
			if mbYesNo("Quit with unsaved buffers?") != PromptResultYes {
				quitRequested = false
				return true
			}
		}
		return false
	}
	return true
}

// drainPendingKeys processes burst input (trackpad scroll, key repeat) before redraw.
// firstKey is the event that unblocked the main loop.
func drainPendingKeys(firstKey uint32) bool {
	wheelNet := 0
	process := func(k uint32) bool {
		if isPasteRedrawKey(k) {
			return true
		}
		if k == MouseWheelUp || k == MouseWheelDown {
			if k == MouseWheelDown {
				wheelNet++
			} else {
				wheelNet--
			}
			return true
		}
		applyWheelTicks(wheelNet)
		wheelNet = 0
		return handleEditorKey(k)
	}
	if !process(firstKey) {
		return false
	}
	for {
		select {
		case k := <-GlobalKeyCh:
			if !process(k) {
				return false
			}
		default:
			applyWheelTicks(wheelNet)
			return true
		}
	}
}

// Run starts the editor.
func Run() {
	// Optional CPU profiling: set JEM_CPU_PROFILE=/tmp/jem-cpu.pprof and optionally
	// JEM_CPU_PROFILE_SECONDS=<n> to capture a <n>-second profile at startup.
	if cpuPath := os.Getenv("JEM_CPU_PROFILE"); cpuPath != "" {
		sec := 10
		if s := os.Getenv("JEM_CPU_PROFILE_SECONDS"); s != "" {
			if v, err := strconv.Atoi(s); err == nil && v > 0 {
				sec = v
			}
		}
		f, err := os.Create(cpuPath)
		if err == nil {
			pprof.StartCPUProfile(f)
			go func() {
				time.Sleep(time.Duration(sec) * time.Second)
				pprof.StopCPUProfile()
				f.Close()
				fmt.Fprintf(os.Stderr, "jem: cpu profile written to %s\n", cpuPath)
			}()
		} else {
			fmt.Fprintf(os.Stderr, "jem: failed to create cpu profile file %s\n", cpuPath)
		}
	}

	// Open terminal and initialize display
	initTermHooks()
	term.Open()
	clipboardReady = clipboard.Init() == nil
	DisplayInit()

	// Ensure terminal is restored on any exit (panic, signal, normal)
	defer term.Close()

	// Load configuration (if present)
	ConfigLoad()

	// Initialize keybindings
	KeybindingsInit()

	// Initialize editor state and create first buffer
	first := "untitled"
	if len(os.Args) > 1 {
		first = os.Args[1]
	}
	EditorInit(first)

	// If filenames were provided, load them into buffers (argv[1] first, then the rest).
	if len(os.Args) > 1 {
		DisplayUpdate()
		loadCommandLineFiles(os.Args[1:])
	}

	// Set up signal handling to request quit on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Key input channels (reader runs in goroutine). Global channels are used so
	// the minibuffer prompt can receive keys directly while active.
	initUIInputChannels()
	backgroundJobsInit()
	startKeyReader()

	DisplayUpdate()

loop:
	for {
		DisplayUpdate()
		select {
		case sig := <-sigCh:
			_ = sig
			if anyUnsavedBuffers() {
				if mbYesNo("Quit with unsaved buffers?") != PromptResultYes {
					continue loop
				}
			}
			break loop
		case done := <-backgroundJobDone:
			backgroundJobHandleDone(done)
			fileCheckReload()
		case k := <-GlobalKeyCh:
			if isPasteRedrawKey(k) {
				continue loop
			}
			if !drainPendingKeys(k) {
				break loop
			}
			fileCheckReload()
		}
	}

	// Normal exit — defer handles term.Close()
}
