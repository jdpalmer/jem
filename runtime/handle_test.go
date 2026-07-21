package runtime

import (
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
	"testing"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/term"
)

func TestHandleKeyRoutesToLegacyDispatch(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("hi")
	if !Handle(State, event.KeyEvent{Code: 'x'}) {
		t.Fatal("Handle returned false")
	}
	te.ExpectText("hix")
}

func TestYesNoListenerConsumesAndPops(t *testing.T) {
	_ = NewTestEditor(t)
	called := false
	AskYesNo("Test?", func() { called = true }, nil)
	if len(listenerStack) != 1 {
		t.Fatalf("stack len = %d, want 1", len(listenerStack))
	}
	if !Handle(State, event.KeyEvent{Code: 'y'}) {
		t.Fatal("Handle returned false")
	}
	if !called {
		t.Fatal("onYes not called")
	}
	if len(listenerStack) != 0 {
		t.Fatalf("stack len = %d after answer, want 0", len(listenerStack))
	}
}

func TestAskStringCompletesOnEnter(t *testing.T) {
	_ = NewTestEditor(t)
	event.DrainForTest()
	EnsureServices()

	done := make(chan string, 1)
	AskString("Name: ", "", func(text string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes {
			t.Errorf("pr = %v, want Yes", pr)
		}
		done <- text
	})
	if len(listenerStack) != 1 {
		t.Fatalf("stack len = %d, want 1", len(listenerStack))
	}

	for _, k := range []uint32{'h', 'i', '\r'} {
		if !Handle(State, event.KeyEvent{Code: k}) {
			t.Fatal("Handle returned false")
		}
	}
	select {
	case got := <-done:
		if got != "hi" {
			t.Fatalf("text = %q, want hi", got)
		}
	default:
		t.Fatal("onDone not called")
	}
	if len(listenerStack) != 0 {
		t.Fatalf("stack len = %d after done, want 0", len(listenerStack))
	}
}

func TestUniversalArgListenerDispatchesWithN(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("")
	event.DrainForTest()

	// C-u 3 x → insert xxx (self-insert with n=3)
	if !Handle(State, event.KeyEvent{Code: term.CTL | 'U'}) {
		t.Fatal("Handle C-u failed")
	}
	if len(listenerStack) != 1 {
		t.Fatalf("stack len = %d after C-u, want 1", len(listenerStack))
	}
	if !Handle(State, event.KeyEvent{Code: '3'}) {
		t.Fatal("Handle 3 failed")
	}
	if !Handle(State, event.KeyEvent{Code: 'x'}) {
		t.Fatal("Handle x failed")
	}
	if len(listenerStack) != 0 {
		t.Fatalf("stack len = %d after dispatch, want 0", len(listenerStack))
	}
	te.ExpectText("xxx")
}

func TestCommandEventRunsNamedCommand(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("hello")
	wp := window.Active.CurrentWindow
	wp.SetCursor(buffer.MakeLocation(1, 0))
	if !Handle(State, event.CommandEvent{Name: "kill", F: false, N: 1}) {
		t.Fatal("Handle CommandEvent failed")
	}
	te.ExpectText("")
}

func TestQuoteListenerInsertsNextKey(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("")
	if !CmdQuote(false, 3) {
		t.Fatal("CmdQuote failed")
	}
	if len(listenerStack) != 1 {
		t.Fatalf("stack len = %d, want 1", len(listenerStack))
	}
	if !Handle(State, event.KeyEvent{Code: 'x'}) {
		t.Fatal("Handle returned false")
	}
	if len(listenerStack) != 0 {
		t.Fatalf("stack len = %d after quote, want 0", len(listenerStack))
	}
	te.ExpectText("xxx")
}

func TestQuoteDuringMacroPlayConsumesNextStep(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("")
	resetMacroState()
	State.Macro = []event.Event{
		event.MacroStepEvent{Code: term.CTL | 'Q', F: false, N: 1},
		event.MacroStepEvent{Code: 'z', F: false, N: 1},
	}
	if !CmdMacroExec(false, 1) {
		t.Fatal("macro play failed")
	}
	te.ExpectText("z")
}
