package display

import "github.com/jdpalmer/jem/buffer"

const (
	Version = "26.1"
	// PatternCapacity is the soft max for search/prompt text and the size of
	// fixed pattern scratch buffers in isearch.
	PatternCapacity = 256
)

type ScreenCoord struct {
	Row int
	Col int
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
	GoalCol            int
	FillCol            int
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

// Bind sets the active display state to the given state.
func Bind(s *State) {
	Active = s
}

// Reset rebinds Active to the package default state and clears it.
func Reset() {
	defaultState = State{}
	Active = &defaultState
}
