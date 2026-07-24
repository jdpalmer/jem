package tools

// Build command runner and diagnostic buffer.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/markring"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
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
	Line     int
	Column   int
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

func compileParseLineColumn(rest string) (line, column int, ok bool) {
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		line = line*10 + int(rest[i]-'0')
		i++
	}
	if line == 0 || i >= len(rest) || rest[i] != ':' {
		return 0, 0, false
	}
	i++
	parsedCol := 0
	hasCol := false
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		parsedCol = parsedCol*10 + int(rest[i]-'0')
		hasCol = true
		i++
	}
	column = 1
	if hasCol && parsedCol > 0 && i < len(rest) && rest[i] == ':' {
		column = parsedCol
	}
	return line, column, true
}

func compileAppendTextSection(buf *buffer.Buffer, title, text string, counts *compileDiagCounts) bool {
	heading := "## " + title
	if buf.AppendLineBytes([]byte(heading)) == nil {
		return false
	}
	if text == "" {
		return buf.AppendLineBytes(nil) != nil
	}
	for _, raw := range strings.Split(text, "\n") {
		textLine := strings.TrimSuffix(raw, "\r")
		line := buf.AppendLineBytes([]byte(textLine))
		if line == nil {
			return false
		}
		if len(textLine) > 0 {
			if diag := compileParseColonDiag(textLine); diag != nil {
				counts.diag++
				switch diag.Severity {
				case CompileDiagError:
					counts.errors++
				case CompileDiagWarning:
					counts.warnings++
				}
				line.Metadata = diag
			}
		}
	}
	return true
}

type compileDiagCounts struct {
	diag, errors, warnings uint32
}

func compileFillBuffer(buf *buffer.Buffer, command, stdout, stderr string, exitCode int, outTrunc, errTrunc bool) (compileDiagCounts, bool) {
	counts := compileDiagCounts{}
	buf.Clear()
	buf.IsChanged = false
	buf.FileName = ""
	buf.FileModTime = time.Time{}
	buf.LangMode = buffer.LModeMarkdown

	cmdLine := "$ " + command
	if buf.AppendLineBytes([]byte(cmdLine)) == nil {
		return counts, false
	}
	if buf.AppendLineBytes(nil) == nil {
		return counts, false
	}
	if !compileAppendTextSection(buf, "stdout", stdout, &counts) {
		return counts, false
	}
	if buf.AppendLineBytes(nil) == nil {
		return counts, false
	}
	if !compileAppendTextSection(buf, "stderr", stderr, &counts) {
		return counts, false
	}
	if outTrunc || errTrunc {
		if buf.AppendLineBytes(nil) == nil {
			return counts, false
		}
		msg := "[output truncated]"
		if buf.AppendLineBytes([]byte(msg)) == nil {
			return counts, false
		}
	}

	summary := fmt.Sprintf("# compile exit=%d, diagnostics=%d, errors=%d, warnings=%d",
		exitCode, counts.diag, counts.errors, counts.warnings)
	summaryLine := buf.Line(1)
	if summaryLine == nil {
		return counts, false
	}
	begin := buffer.MakeLocation(1, 0)
	end := buffer.MakeLocation(1, summaryLine.Len())
	if err := buf.SetText(nil, begin, end, []byte(summary), nil); err != nil {
		return counts, false
	}

	buf.IsChanged = false
	buf.Cursor = buffer.Location{Line: 1, Offset: 0}
	buf.Mark = buffer.Location{Line: 0, Offset: 0}
	return counts, true
}

func compileEnsureBuffer() *buffer.Buffer {
	if buf := buffer.Find(CompileBufferName); buf != nil {
		return buf
	}
	buf := buffer.Create()
	if buf == nil {
		return nil
	}
	buf.Name = CompileBufferName
	return buf
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
	var out bytes.Buffer
	limited := io.LimitReader(r, int64(max)+1)
	_, _ = io.Copy(&out, limited)
	data := out.Bytes()
	truncated := len(data) > max
	if truncated {
		data = data[:max]
	}
	return string(data), truncated
}

// RunCompile runs a shell build command and captures output in *compile*.
func RunCompile() bool {
	if PackageHooks.AskString != nil {
		PackageHooks.AskString("Compile: ", compileLastCommand, func(command string, pr minibuffer.PromptResult) {
			if pr != minibuffer.PromptResultYes {
				return
			}
			if command == "" {
				display.MBWrite("[empty compile command]")
				return
			}
			display.MBHistoryAdd(command)
			compileLastCommand = command
			_ = StartBackgroundCompile(command)
		})
	}
	return true
}

// VisitCompileDiag jumps to the diagnostic at the current line in *compile*.
func VisitCompileDiag() bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.Name != CompileBufferName {
		return false
	}
	line := buf.Line(win.Cursor.Line)
	if line == nil || line.Len() == 0 || line.Metadata == nil {
		return false
	}
	data, ok := line.Metadata.(*CompileLineData)
	if !ok || data == nil || data.Path == "" || data.Line == 0 {
		return false
	}

	markring.PushCurrent()
	if PackageHooks.VisitLocation == nil {
		return false
	}
	return PackageHooks.VisitLocation(data.Path, data.Line, data.Column)
}
