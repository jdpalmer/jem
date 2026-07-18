package app

import "github.com/jdpalmer/jem/buffer"

const MarkCapacity = 32

type Mark struct {
	Buffer        *buffer.Buffer
	BufferSerial  uint32
	LineNumber    uint
	Offset        uint
	TopLineNumber uint
	HScroll       uint32
}

type MarkState struct {
	Marks []Mark
}

var MarksState MarkState

func markUpdateModelines() {
	for _, wp := range State.WINDOWS {
		if wp != nil {
			wp.ShouldUpdateModeLine = true
		}
	}
}

func markEquals(a, b *Mark) bool {
	return a.Buffer == b.Buffer &&
		a.BufferSerial == b.BufferSerial &&
		a.LineNumber == b.LineNumber &&
		a.Offset == b.Offset &&
		a.TopLineNumber == b.TopLineNumber &&
		a.HScroll == b.HScroll
}

func markCaptureCurrent(m *Mark) bool {
	wp := State.CurrentWindow
	bp := State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	m.Buffer = bp
	m.BufferSerial = bp.Serial
	m.LineNumber = wp.Cursor.Line
	m.Offset = wp.Cursor.Offset
	m.TopLineNumber = wp.TopLine
	m.HScroll = wp.HScroll
	return true
}

func marksPushEntry(mark *Mark) {
	if len(MarksState.Marks) == MarkCapacity {
		copy(MarksState.Marks[0:], MarksState.Marks[1:])
		MarksState.Marks = MarksState.Marks[:MarkCapacity-1]
	}
	MarksState.Marks = append(MarksState.Marks, *mark)
}

func markBufferIsActive(buf *buffer.Buffer, serial uint32) bool {
	for _, bp := range State.Buffers {
		if bp == buf && bp.Serial == serial {
			return true
		}
	}
	return false
}

func MarkPushCurrent() {
	var mark Mark
	if !markCaptureCurrent(&mark) {
		return
	}
	if n := len(MarksState.Marks); n > 0 && markEquals(&MarksState.Marks[n-1], &mark) {
		return
	}
	marksPushEntry(&mark)
	markUpdateModelines()
}

func markRestore(m *Mark) bool {
	if m.Buffer == nil || !markBufferIsActive(m.Buffer, m.BufferSerial) {
		return false
	}
	if State.CurrentBuffer != m.Buffer {
		if PackageHooks.SwitchBuffer != nil {
			PackageHooks.SwitchBuffer(m.Buffer)
		} else {
			SetCurrentBuffer(m.Buffer)
			if State.CurrentWindow != nil {
				State.CurrentWindow.Buffer = m.Buffer
			}
		}
	}
	wp := State.CurrentWindow
	if wp == nil {
		return false
	}
	lp := m.Buffer.Line(m.LineNumber)
	offset := m.Offset
	if lp != nil && offset > lp.Len() {
		offset = lp.Len()
	}
	wp.SetCursor(buffer.MakeLocation(m.LineNumber, offset))
	wp.SetTopLine(m.TopLineNumber)
	wp.HScroll = m.HScroll
	wp.DidMove = true
	wp.ShouldRedraw = true
	wp.ShouldUpdateModeLine = true
	return true
}

func MarkPopOnce() bool {
	for len(MarksState.Marks) > 0 {
		n := len(MarksState.Marks)
		mark := MarksState.Marks[n-1]
		MarksState.Marks = MarksState.Marks[:n-1]
		if markRestore(&mark) {
			markUpdateModelines()
			return true
		}
	}
	markUpdateModelines()
	return false
}
