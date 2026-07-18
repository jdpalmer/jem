package editor

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/tools"
)

const (
	grepBufferName    = tools.GrepBufferName
	compileBufferName = tools.CompileBufferName
)

type (
	GrepLineData      = tools.GrepLineData
	CompileLineData   = tools.CompileLineData
	BackgroundJobDone = tools.BackgroundJobDone
)

var backgroundJobDone <-chan BackgroundJobDone

func backgroundJobsInit() {
	tools.InitBackgroundJobs()
	backgroundJobDone = tools.BackgroundJobDoneChan()
}

func backgroundJobHandleDone(done BackgroundJobDone) {
	tools.HandleBackgroundJobDone(done)
}

func backgroundJobRunning() bool {
	return tools.BackgroundJobRunning()
}

func backgroundJobRequestCancel() bool {
	return tools.RequestBackgroundJobCancel()
}

func CmdGrepProject(_ bool, _ int) bool {
	return tools.RunGrep()
}

func CmdGrepVisitMatch(_ bool, _ int) bool {
	return tools.VisitGrepMatch()
}

func CmdCompile(_ bool, _ int) bool {
	return tools.RunCompile()
}

func CmdCompileVisitDiag(_ bool, _ int) bool {
	return tools.VisitCompileDiag()
}

func CmdGotoTag(_ bool, _ int) bool {
	return tools.RunGotoTag()
}

func tagsMaybeShowCallHint() {
	tools.MaybeShowCallHint()
}

func CmdSpawnCli(_ bool, _ int) bool {
	return tools.RunSpawnCLI()
}

func CmdSpawn(_ bool, _ int) bool {
	return tools.RunSpawnCommand()
}

func gitModelineText(bp *buffer.Buffer) string {
	return tools.GitModelineText(bp)
}

func gitLineDiff(bp *buffer.Buffer, lineNumber uint) app.GitLineDiff {
	return tools.GitLineDiffAt(bp, lineNumber)
}
