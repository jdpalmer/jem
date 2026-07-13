package editor

// keybindings.go - Key dispatch and command registry initialization.

// RegisterCommands populates commandNameMap and keybindingsMap from commandTable.
func RegisterCommands() {
	initCommandRegistry()
}

func KeybindingsInit() {
	initCommandRegistry()
}

func DispatchCommand(keycode uint32) bool {
	return processEditorKey(keycode)
}

// CmdQuit requests the editor to quit. It sets a global flag observed by the
// main loop.
func CmdQuit(f bool, n int) bool {
	_ = f
	_ = n
	quitRequested = true
	return true
}
