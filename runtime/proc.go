package runtime

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/window"
)

const (
	MacroCapacity   = 256
	PatternCapacity = display.PatternCapacity
)

type ProcState struct {
	Dispatching       bool
	Macro             []event.Event
	PlayPos           int
	WhitespaceCleanup bool
	AutoRevertMode    bool
	Indent            buffer.IndentConfig
}

var defaultState ProcState = ProcState{PlayPos: -1}
var State *ProcState = &defaultState

func BindState(s *ProcState) {
	State = s
}

func Reset() {
	*State = ProcState{PlayPos: -1}
	*History = buffer.UndoHistory{}
	*buffer.All = buffer.List{}
	*window.Active = window.State{}
	display.Reset()
}

func (s *ProcState) IsRecording() bool { return display.Active.MacroRecording }
func (s *ProcState) IsPlaying() bool   { return s.PlayPos >= 0 }
func (s *ProcState) HasMacro() bool    { return display.Active.MacroPresent }
