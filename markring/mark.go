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
	SwitchBuffer     func(bp *buffer.Buffer)
	SetCurrentBuffer func(bp *buffer.Buffer)
}

// PackageHooks is set by the runtime during init.
var PackageHooks Hooks

func markUpdateModelines() {
	for _, wp := range window.Active.Windows {
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
	wp := window.Active.CurrentWindow
	var bp *buffer.Buffer
	if PackageHooks.CurrentBuffer != nil {
		bp = PackageHooks.CurrentBuffer()
	}
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
	if len(Active.Marks) == MarkCapacity {
		copy(Active.Marks[0:], Active.Marks[1:])
		Active.Marks = Active.Marks[:MarkCapacity-1]
	}
	Active.Marks = append(Active.Marks, *mark)
}

func markBufferIsActive(buf *buffer.Buffer, serial uint32) bool {
	if PackageHooks.Buffers == nil {
		return false
	}
	for _, bp := range PackageHooks.Buffers() {
		if bp == buf && bp.Serial == serial {
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
	wp := window.Active.CurrentWindow
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
