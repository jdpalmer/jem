package session

const MarkCapacity = 32

type Mark struct {
	Buffer        *Buffer
	BufferSerial  uint32
	LineNumber    uint
	Offset        uint
	TopLineNumber uint
	HScroll       uint32
}

type MarkState struct {
	Marks [MarkCapacity]Mark
	Count uint8
}

var MarksState MarkState

func markUpdateModelines() {
	for i := 0; i < int(App.WindowCount); i++ {
		wp := App.WINDOWS[i]
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
	wp := App.CurrentWindow
	bp := App.CurrentBuffer
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

func marksPushEntry(stack []Mark, count *uint8, mark *Mark) []Mark {
	if int(*count) == MarkCapacity {
		copy(stack[0:], stack[1:])
		*count = MarkCapacity - 1
	}
	stack[*count] = *mark
	*count++
	return stack
}

func markBufferIsActive(buf *Buffer, serial uint32) bool {
	for i := 0; i < int(App.BufferCount); i++ {
		bp := App.Buffers[i]
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
	if MarksState.Count > 0 && markEquals(&MarksState.Marks[MarksState.Count-1], &mark) {
		return
	}
	marksPushEntry(MarksState.Marks[:], &MarksState.Count, &mark)
	markUpdateModelines()
}

func markRestore(m *Mark) bool {
	if m.Buffer == nil || !markBufferIsActive(m.Buffer, m.BufferSerial) {
		return false
	}
	if App.CurrentBuffer != m.Buffer {
		if PackageHooks.SwitchBuffer != nil {
			PackageHooks.SwitchBuffer(m.Buffer)
		} else {
			SetCurrentBuffer(m.Buffer)
			if App.CurrentWindow != nil {
				App.CurrentWindow.Buffer = m.Buffer
			}
		}
	}
	wp := App.CurrentWindow
	if wp == nil {
		return false
	}
	lp := BufferGetLine(m.Buffer, m.LineNumber)
	offset := m.Offset
	if lp != nil && offset > LineLength(lp) {
		offset = LineLength(lp)
	}
	WindowSetCursor(wp, MakeLocation(m.LineNumber, offset))
	WindowSetTopLine(wp, m.TopLineNumber)
	wp.HScroll = m.HScroll
	wp.DidMove = true
	wp.ShouldRedraw = true
	wp.ShouldUpdateModeLine = true
	return true
}

func MarkPopOnce() bool {
	for MarksState.Count > 0 {
		mark := MarksState.Marks[MarksState.Count-1]
		MarksState.Count--
		if markRestore(&mark) {
			markUpdateModelines()
			return true
		}
	}
	markUpdateModelines()
	return false
}
