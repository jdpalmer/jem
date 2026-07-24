package runtime

import (
	"fmt"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/killring"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"syscall"
	"time"

	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/tools"
	"golang.design/x/clipboard"
)

// normalizeCtlxKey adjusts CTLX|CTL|ch to CTLX|ch when only the plain chord is bound.
func normalizeCtlxKey(combined uint32) uint32 {
	if combined&term.CTLX == 0 {
		return combined
	}
	second := combined &^ term.CTLX
	if second&term.CTL == 0 {
		return combined
	}
	plain := term.CTLX | (second & 0xFF)
	if _, hasPlain := keybindingsMap[plain]; hasPlain {
		if _, hasCtrl := keybindingsMap[combined]; !hasCtrl {
			return plain
		}
	}
	return combined
}

// applyCtlxPrefix forms the editor key code for the key following C-x.
func applyCtlxPrefix(second uint32) uint32 {
	return normalizeCtlxKey(term.CTLX | display.DecodeKeyChar(second, true))
}

// editorReadKey reads one editor command key on the main thread.
func editorReadKey(keyOut *uint32) bool {
	k, ok := display.ReadEditorKey()
	if !ok {
		return false
	}
	*keyOut = k
	return true
}

func anyUnsavedBuffers() bool {
	for i := 0; i < len(buffer.All.Buffers); i++ {
		buf := buffer.All.Buffers[i]
		if buf != nil && buf.IsChanged {
			return true
		}
	}
	return false
}

// drainEvents processes queued events until empty. Returns false to exit the runtime.
func drainEvents() bool {
	for {
		select {
		case e := <-event.Chan():
			if !Handle(State, e) {
				return false
			}
		default:
			return true
		}
	}
}

// Run starts the runtime. Pass nil to create a fresh Editor.
func Run(e *App) {
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

	// Open terminal and initialize display
	term.Open()
	killring.ClipboardReady = clipboard.Init() == nil
	display.DisplayInit()

	// Ensure terminal is restored on any exit (panic, signal, normal)
	defer term.Close()

	// Load configuration (if present)
	ConfigLoad()

	// Initialize keybindings
	InitCommands()

	// Initialize editor state and create first buffer
	first := "untitled"
	if len(os.Args) > 1 {
		first = os.Args[1]
	}
	AppInit(first)

	// If filenames were provided, load them into buffers (argv[1] first, then the rest).
	if len(os.Args) > 1 {
		display.DisplayUpdate()
		loadCommandLineFiles(os.Args[1:])
	}

	// Set up signal handling to request quit on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Paste arrives via event.PasteEvent; keys via event.Enqueue.
	// Background jobs post JobDoneEvent directly onto the bus.
	tools.InitBackgroundJobs()
	display.StartKeyReader()

	display.DisplayUpdate()

loop:
	for {
		display.DisplayUpdate()
		select {
		case sig := <-sigCh:
			_ = sig
			event.Enqueue(event.QuitEvent{Force: false})
			if !drainEvents() {
				break loop
			}
		case e := <-event.Chan():
			if !Handle(State, e) {
				break loop
			}
			if !drainEvents() {
				break loop
			}
			fileCheckReload()
		}
	}

	// Normal exit — defer handles term.Close()
}

// CmdQuit requests the editor to quit. It sets a flag observed by the main loop.
func CmdQuit(f bool, n int) bool {
	_ = f
	_ = n
	ensureCurrent().QuitRequested = true
	return true
}
