package markring

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/window"
)

const MarkCapacity = 32

// Mark is one saved window position on the mark ring.
type Mark struct {
	Buffer        *buffer.Buffer
	BufferSerial  uint32
	LineNumber    uint
	Offset        uint
	TopLineNumber uint
	HScroll       uint32
}

// State holds the mark ring.
type State struct {
	Marks []Mark
}

// Active is the process-wide mark ring.
var Active State

// Hooks connects markring to buffer-list helpers it cannot import.
type Hooks struct {
	CurrentBuffer    func() *buffer.Buffer
	Buffers          func() []*buffer.Buffer
	SwitchBuffer     func(buf *buffer.Buffer)
	SetCurrentBuffer func(buf *buffer.Buffer)
}

// PackageHooks is set by the runtime during init.
var PackageHooks Hooks

func markUpdateModelines() {
	for _, win := range window.Active.Windows {
		if win != nil {
			win.ShouldUpdateModeLine = true
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
	win := window.Active.CurrentWindow
	var buf *buffer.Buffer
	if PackageHooks.CurrentBuffer != nil {
		buf = PackageHooks.CurrentBuffer()
	}
	if win == nil || buf == nil {
		return false
	}
	m.Buffer = buf
	m.BufferSerial = buf.Serial
	m.LineNumber = win.Cursor.Line
	m.Offset = win.Cursor.Offset
	m.TopLineNumber = win.TopLine
	m.HScroll = win.HScroll
	return true
}

func marksPushEntry(mark *Mark) {
	if len(Active.Marks) == MarkCapacity {
		copy(Active.Marks[0:], Active.Marks[1:])
		Active.Marks = Active.Marks[:MarkCapacity-1]
	}
	Active.Marks = append(Active.Marks, *mark)
}

func markBufferIsActive(candidate *buffer.Buffer, serial uint32) bool {
	if PackageHooks.Buffers == nil {
		return false
	}
	for _, buf := range PackageHooks.Buffers() {
		if buf == candidate && buf.Serial == serial {
			return true
		}
	}
	return false
}

// PushCurrent pushes the current window position onto the mark ring.
func PushCurrent() {
	var mark Mark
	if !markCaptureCurrent(&mark) {
		return
	}
	if n := len(Active.Marks); n > 0 && markEquals(&Active.Marks[n-1], &mark) {
		return
	}
	marksPushEntry(&mark)
	markUpdateModelines()
}

func markRestore(m *Mark) bool {
	if m.Buffer == nil || !markBufferIsActive(m.Buffer, m.BufferSerial) {
		return false
	}
	var cur *buffer.Buffer
	if PackageHooks.CurrentBuffer != nil {
		cur = PackageHooks.CurrentBuffer()
	}
	if cur != m.Buffer {
		if PackageHooks.SwitchBuffer != nil {
			PackageHooks.SwitchBuffer(m.Buffer)
		} else if PackageHooks.SetCurrentBuffer != nil {
			PackageHooks.SetCurrentBuffer(m.Buffer)
			if window.Active.CurrentWindow != nil {
				window.Active.CurrentWindow.Buffer = m.Buffer
			}
		}
	}
	win := window.Active.CurrentWindow
	if win == nil {
		return false
	}
	line := m.Buffer.Line(m.LineNumber)
	offset := m.Offset
	if line != nil && offset > line.Len() {
		offset = line.Len()
	}
	win.SetCursor(buffer.MakeLocation(m.LineNumber, offset))
	win.SetTopLine(m.TopLineNumber)
	win.HScroll = m.HScroll
	win.DidMove = true
	win.ShouldRedraw = true
	win.ShouldUpdateModeLine = true
	return true
}

// PopOnce pops and restores the most recent valid mark.
func PopOnce() bool {
	for len(Active.Marks) > 0 {
		n := len(Active.Marks)
		mark := Active.Marks[n-1]
		Active.Marks = Active.Marks[:n-1]
		if markRestore(&mark) {
			markUpdateModelines()
			return true
		}
	}
	markUpdateModelines()
	return false
}
