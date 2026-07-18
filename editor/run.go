package editor

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"syscall"
	"time"

	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/edit"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/ui"
	"golang.design/x/clipboard"
)

// applyCtlxPrefix forms the editor key code for the key following C-x.
// Mirrors C main.c: c = CTLX | decode_key_char(control_key, true).
func applyCtlxPrefix(second uint32) uint32 {
	second = ui.DecodeKeyChar(second, true)
	combined := term.CTLX | second
	// Chords like C-x b are bound as CTLX|'B'; if Ctrl stays held on the
	// second key, normalize CTLX|CTL|'B' to the plain binding when needed.
	if second&term.CTL != 0 {
		plain := term.CTLX | (second & 0xFF)
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
		k = ui.DecodeKeyChar(k, false)
		if !editorEscapePrefixPending {
			if k == 0x1B {
				editorEscapePrefixPending = true
				ui.MBWrite("ESC")
				continue
			}
			*keyOut = k
			return true
		}
		editorEscapePrefixPending = false
		*keyOut = ui.ApplyMetaPrefixToKey(k)
		return true
	}
}

func anyUnsavedBuffers() bool {
	for i := 0; i < int(len(app.State.Buffers)); i++ {
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
		ui.MBClear()
	}
	if k == 0x03 { // Ctrl-C
		if anyUnsavedBuffers() {
			if ui.MBYesNo("Quit with unsaved buffers?") != app.PromptResultYes {
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
	if Current != nil && Current.QuitRequested {
		if anyUnsavedBuffers() {
			if ui.MBYesNo("Quit with unsaved buffers?") != app.PromptResultYes {
				Current.QuitRequested = false
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
		if ui.IsPasteRedrawKey(k) {
			return true
		}
		if k == term.MouseWheelUp || k == term.MouseWheelDown {
			if k == term.MouseWheelDown {
				wheelNet++
			} else {
				wheelNet--
			}
			return true
		}
		ui.ApplyWheelTicks(wheelNet)
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
			ui.ApplyWheelTicks(wheelNet)
			return true
		}
	}
}

// Run starts the editor. Pass nil to create a fresh Editor.
func Run(e *Editor) {
	if e == nil {
		e = New()
	}
	e.Activate()

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

	// Open terminal and initialize display (term hooks before Open for paste/mouse)
	EnsureServices()
	term.Open()
	edit.ClipboardReady = clipboard.Init() == nil
	ui.DisplayInit()

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
		ui.DisplayUpdate()
		loadCommandLineFiles(os.Args[1:])
	}

	// Set up signal handling to request quit on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Key input channels (reader runs in goroutine). Global channels are used so
	// the minibuffer prompt can receive keys directly while active.
	GlobalKeyCh = make(chan uint32, 64)
	GlobalMinibufKeyCh = make(chan uint32, 16)
	ui.InitInputChannels(GlobalKeyCh, GlobalMinibufKeyCh, 4)
	backgroundJobsInit()
	ui.StartKeyReader()

	ui.DisplayUpdate()

loop:
	for {
		ui.DisplayUpdate()
		select {
		case sig := <-sigCh:
			_ = sig
			if anyUnsavedBuffers() {
				if ui.MBYesNo("Quit with unsaved buffers?") != app.PromptResultYes {
					continue loop
				}
			}
			break loop
		case done := <-backgroundJobDone:
			backgroundJobHandleDone(done)
			fileCheckReload()
		case k := <-GlobalKeyCh:
			if ui.IsPasteRedrawKey(k) {
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
