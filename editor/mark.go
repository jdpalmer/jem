package editor

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/ui"
)

const MarkCapacity = app.MarkCapacity

type Mark = app.Mark
type MarkState = app.MarkState

var marksState = &app.MarksState

func markPopOnce() bool {
	if !app.MarkPopOnce() {
		ui.MBWrite("[no saved mark]")
		return false
	}
	return true
}

// CmdMarkPush saves the current location on the mark stack.
func CmdMarkPush(f bool, n int) bool {
	_ = f
	_ = n
	app.MarkPushCurrent()
	ui.MBWrite("[mark pushed]")
	return true
}

// CmdMarkPop restores the most recently pushed mark.
func CmdMarkPop(f bool, n int) bool {
	_ = f
	_ = n
	return markPopOnce()
}
