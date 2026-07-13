package ui

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
	sel := uint8(0)
	for {
		result := mbChoose("Menu > ", mainMenu, menuItemLabel, uint8(len(mainMenu)), sel)
		if result == -2 {
			if PackageHooks.Abort != nil {
				PackageHooks.Abort()
			}
			return false
		}
		if result < 0 {
			mbClear()
			return false
		}
		sel = uint8(result)
		cmd := mainMenu[result].command
		mbClear()
		if PackageHooks.RunCommandByName == nil {
			return false
		}
		return PackageHooks.RunCommandByName(cmd)
	}
}
