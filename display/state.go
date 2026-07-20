package display

import "github.com/jdpalmer/jem/buffer"

const (
	Version         = "26.1"
	PatternCapacity = 256
)

type ScreenCoord struct {
	Row uint32
	Col uint32
}

type ThemeMode int

const (
	ThemeDark ThemeMode = iota
	ThemeLight
)

type ThemeState struct {
	NormalStyle          buffer.TextStyle
	CommentStyle         buffer.TextStyle
	PickerSelectionStyle buffer.TextStyle
	GutterStyle          buffer.TextStyle
	SelectionBg          buffer.TermColor
	ModelineNameColor    buffer.TermColor
	Mode                 ThemeMode
}

// State is the display-owned portion of active editor state.
type State struct {
	Cursor             ScreenCoord
	PhantomCursor      ScreenCoord
	GoalCol            uint32
	FillCol            uint32
	Theme              ThemeState
	PhantomText        byte
	MessagePresent     bool
	PhantomCursorValid bool
	ShowPhantomCursor  bool
	ScreenDirty        bool
	PhantomStyle       buffer.TextStyle
	ActiveStyle        buffer.TextStyle
	Mouse              ScreenCoord
	MacroRecording     bool
	MacroPlaying       bool
	MacroPresent       bool
}

var defaultState State
var Active *State = &defaultState

func Bind(s *State) {
	if s == nil {
		Active = &defaultState
		return
	}
	Active = s
}

func Reset() { *Active = State{} }
