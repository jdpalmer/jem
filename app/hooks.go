package app

type Hooks struct {
	UndoForgetBuffer func(bp *Buffer)
	SetCurrentBuffer func(bp *Buffer)
	SwitchBuffer     func(bp *Buffer)
}

var PackageHooks Hooks
