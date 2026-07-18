package tools

// compile.go — build command runner and diagnostic buffer (translation of src/cmd_compile.c)

import (
	"bytes"
	"context"
	"fmt"
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
)

const (
	CompileBufferName  = "*compile*"
	compileOutCapacity = 1024 * 1024
	compileErrCapacity = 512 * 1024
)

var compileLastCommand = "make -k"

type CompileDiagSeverity int

const (
	CompileDiagUnknown CompileDiagSeverity = iota
	CompileDiagError
	CompileDiagWarning
	CompileDiagNote
)

// CompileLineData is stored on diagnostic lines in the *compile* buffer.
type CompileLineData struct {
	Path     string
	Line     uint32
	Column   uint32
	Severity CompileDiagSeverity
}

func compileClassifySeverity(text string) CompileDiagSeverity {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "error") {
		return CompileDiagError
	}
	if strings.Contains(lower, "warning") {
		return CompileDiagWarning
	}
	if strings.Contains(lower, "note") {
		return CompileDiagNote
	}
	return CompileDiagUnknown
}

func compileParseColonDiag(text string) *CompileLineData {
	if text == "" {
		return nil
	}
	start := 0
	if runtime.GOOS == "windows" && len(text) >= 2 &&
		unicode.IsLetter(rune(text[0])) && text[1] == ':' {
		start = 2
	}
	pathEnd := start
	for pathEnd < len(text) && text[pathEnd] != ':' {
		pathEnd++
	}
	if pathEnd >= len(text) || pathEnd == start {
		return nil
	}
	rest := text[pathEnd+1:]
	line, col, ok := compileParseLineColumn(rest)
	if !ok {
		return nil
	}
	return &CompileLineData{
		Path:     filepath.Clean(text[start:pathEnd]),
		Line:     line,
		Column:   col,
		Severity: compileClassifySeverity(text),
	}
}

func compileParseLineColumn(rest string) (line, column uint32, ok bool) {
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		line = line*10 + uint32(rest[i]-'0')
		i++
	}
	if line == 0 || i >= len(rest) || rest[i] != ':' {
		return 0, 0, false
	}
	i++
	colStart := i
	parsedCol := uint32(0)
	hasCol := false
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		parsedCol = parsedCol*10 + uint32(rest[i]-'0')
		hasCol = true
		i++
	}
	column = 1
	if hasCol && parsedCol > 0 && i < len(rest) && rest[i] == ':' {
		column = parsedCol
		_ = colStart
	}
	return line, column, true
}

func compileAppendTextSection(bp *buffer.Buffer, title, text string, counts *compileDiagCounts) bool {
	heading := "## " + title
	if bp.AppendLineBytes([]byte(heading)) == nil {
		return false
	}
	if text == "" {
		return bp.AppendLineBytes(nil) != nil
	}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSuffix(raw, "\r")
		lp := bp.AppendLineBytes([]byte(line))
		if lp == nil {
			return false
		}
		if len(line) > 0 {
			if diag := compileParseColonDiag(line); diag != nil {
				counts.diag++
				switch diag.Severity {
				case CompileDiagError:
					counts.errors++
				case CompileDiagWarning:
					counts.warnings++
				}
				lp.Metadata = diag
			}
		}
	}
	return true
}

type compileDiagCounts struct {
	diag, errors, warnings uint32
}

func compileFillBuffer(bp *buffer.Buffer, command, stdout, stderr string, exitCode int, outTrunc, errTrunc bool) (compileDiagCounts, bool) {
	counts := compileDiagCounts{}
	bp.Clear()
	bp.IsChanged = false
	bp.FileName = ""
	bp.FileMtime = time.Time{}
	bp.LangMode = buffer.LModeMarkdown

	if bp.AppendLineBytes(nil) == nil {
		return counts, false
	}
	cmdLine := "$ " + command
	if bp.AppendLineBytes([]byte(cmdLine)) == nil {
		return counts, false
	}
	if bp.AppendLineBytes(nil) == nil {
		return counts, false
	}
	if !compileAppendTextSection(bp, "stdout", stdout, &counts) {
		return counts, false
	}
	if bp.AppendLineBytes(nil) == nil {
		return counts, false
	}
	if !compileAppendTextSection(bp, "stderr", stderr, &counts) {
		return counts, false
	}
	if outTrunc || errTrunc {
		if bp.AppendLineBytes(nil) == nil {
			return counts, false
		}
		msg := "[output truncated]"
		if bp.AppendLineBytes([]byte(msg)) == nil {
			return counts, false
		}
	}

	summary := fmt.Sprintf("# compile exit=%d, diagnostics=%d, errors=%d, warnings=%d",
		exitCode, counts.diag, counts.errors, counts.warnings)
	summaryLine := bp.Line(1)
	if summaryLine == nil {
		return counts, false
	}
	begin := buffer.MakeLocation(1, 0)
	end := buffer.MakeLocation(1, summaryLine.Len())
	if err := bp.SetText(nil, begin, end, []byte(summary), nil); err != nil {
		return counts, false
	}

	bp.IsChanged = false
	bp.Cursor = buffer.Location{Line: 1, Offset: 0}
	bp.Mark = buffer.Location{Line: 0, Offset: 0}
	return counts, true
}

func compileEnsureBuffer() *buffer.Buffer {
	if bp := app.BufferFind(CompileBufferName); bp != nil {
		return bp
	}
	bp := app.BufferCreate(&app.State.EditorRuntimeState)
	if bp == nil {
		return nil
	}
	bp.Name = CompileBufferName
	return bp
}

func procRunShellContext(ctx context.Context, command string, outMax, errMax int) (stdout, stderr string, exitCode int, ran bool, outTrunc, errTrunc bool) {
	if command == "" {
		return "", "", -1, false, false, false
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		shell := os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe"
		}
		cmd = exec.CommandContext(ctx, shell, "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", -1, false, false, false
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", "", -1, false, false, false
	}
	if err := cmd.Start(); err != nil {
		return "", "", -1, false, false, false
	}
	stdout, outTrunc = readProcessStream(stdoutPipe, outMax)
	stderr, errTrunc = readProcessStream(stderrPipe, errMax)
	err = cmd.Wait()
	if ctx.Err() != nil {
		return stdout, stderr, -1, true, outTrunc, errTrunc
	}
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", "", -1, false, outTrunc, errTrunc
		}
	}
	return stdout, stderr, exitCode, true, outTrunc, errTrunc
}

func readProcessStream(r io.Reader, max int) (string, bool) {
	if max <= 0 {
		return "", false
	}
	var buf bytes.Buffer
	limited := io.LimitReader(r, int64(max)+1)
	_, _ = io.Copy(&buf, limited)
	data := buf.Bytes()
	truncated := len(data) > max
	if truncated {
		data = data[:max]
	}
	return string(data), truncated
}

// RunCompile runs a shell build command and captures output in *compile*.
func RunCompile() bool {
	command, pr := mbReadString("Compile: ", compileLastCommand)
	if pr != app.PromptResultYes {
		return false
	}
	if command == "" {
		mbWrite("[empty compile command]")
		return false
	}
	mbHistoryAdd(command)
	compileLastCommand = command

	return StartBackgroundCompile(command)
}

// VisitCompileDiag jumps to the diagnostic at the current line in *compile*.
func VisitCompileDiag() bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.Name != CompileBufferName {
		return false
	}
	lp := bp.Line(wp.Cursor.Line)
	if lp == nil || lp.Len() == 0 || lp.Metadata == nil {
		return false
	}
	data, ok := lp.Metadata.(*CompileLineData)
	if !ok || data == nil || data.Path == "" || data.Line == 0 {
		return false
	}

	markPushCurrent()
	return fileVisitLocation(data.Path, data.Line, data.Column)
}
