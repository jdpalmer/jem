package files

// Hooks are runtime-owned process settings files cannot import directly (cycle).
type Hooks struct {
	IsDispatching  func() bool
	AutoRevertMode func() bool
}

// PackageHooks is set once via runtime.Services.
var PackageHooks Hooks

func isDispatching() bool {
	if PackageHooks.IsDispatching == nil {
		return false
	}
	return PackageHooks.IsDispatching()
}

func autoRevertMode() bool {
	if PackageHooks.AutoRevertMode == nil {
		return false
	}
	return PackageHooks.AutoRevertMode()
}
