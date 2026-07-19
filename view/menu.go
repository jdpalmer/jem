package view

type menuItem struct {
	label   string
	command string
}

func menuItemLabel(ctx any, idx uint8) []byte {
	items := ctx.([]menuItem)
	if int(idx) >= len(items) {
		return nil
	}
	return []byte(items[idx].label)
}

var mainMenu = []menuItem{
	{label: "Open", command: "file_visit"},
	{label: "Save", command: "file_save"},
	{label: "Undo", command: "undo"},
	{label: "Yank", command: "yank"},
	{label: "Search", command: "isearch_forward"},
	{label: "Menu Quit", command: "quit"},
}

// CmdMenuRun opens the message-line menu and dispatches via hooks.
func CmdMenuRun(f bool, n int) bool {
	_ = f
	_ = n
	AskChoose("Menu > ", mainMenu, menuItemLabel, uint8(len(mainMenu)), 0, func(result int16) {
		if result == -2 {
			if PackageHooks.Abort != nil {
				PackageHooks.Abort()
			}
			return
		}
		if result < 0 {
			MBClear()
			return
		}
		cmd := mainMenu[result].command
		MBClear()
		if PackageHooks.RunCommandByName != nil {
			PackageHooks.RunCommandByName(cmd)
		}
	})
	return true
}
