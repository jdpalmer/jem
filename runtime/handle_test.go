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
	win := window.Active.CurrentWindow
	win.SetCursor(buffer.MakeLocation(1, 0))
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

func TestConsumedAndPopKeepsListenerPushedFromOnDone(t *testing.T) {
	_ = NewTestEditor(t)
	clearListeners()
	t.Cleanup(clearListeners)

	var stub textPrompt = &stringPromptStub{}
	PushListener(&promptListener{
		prompt: stub,
		onDone: func(string, minibuffer.PromptResult) {
			AskYesNo("Quit with unsaved buffers?", nil, nil)
		},
	})
	if len(listenerStack) != 1 {
		t.Fatalf("stack len = %d, want 1", len(listenerStack))
	}
	if !Handle(State, event.KeyEvent{Code: '\r'}) {
		t.Fatal("Handle returned false")
	}
	if len(listenerStack) != 1 {
		t.Fatalf("stack len = %d after pop, want 1 (yes/no)", len(listenerStack))
	}
	if _, ok := listenerStack[0].(*yesNoListener); !ok {
		t.Fatalf("top listener = %T, want *yesNoListener", listenerStack[0])
	}
}

// stringPromptStub completes on Enter with Yes.
type stringPromptStub struct{}

func (s *stringPromptStub) HandleKey(k uint32) (bool, string, minibuffer.PromptResult) {
	if k == '\r' || k == '\n' {
		return true, "quit", minibuffer.PromptResultYes
	}
	return false, "", 0
}

func (s *stringPromptStub) Close() {}

func TestInvokeCommandQuitShowsUnsavedPrompt(t *testing.T) {
	te := NewTestEditor(t)
	clearListeners()
	t.Cleanup(clearListeners)
	te.BP().IsChanged = true
	ensureCurrent().QuitRequested = false

	_ = invokeCommand(CmdQuit, false, 1)
	if len(listenerStack) != 1 {
		t.Fatalf("stack len = %d, want 1 yes/no after Quit", len(listenerStack))
	}
	if _, ok := listenerStack[0].(*yesNoListener); !ok {
		t.Fatalf("top = %T, want *yesNoListener", listenerStack[0])
	}
}

