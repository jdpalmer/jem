package buffer

import "bytes"

// UndoKind describes how to reverse a recorded edit during replay:
//   - UndoDelete: the forward edit removed text; undo re-inserts record.Text.
//   - UndoInsert: the forward edit added text; undo deletes record.Text.
type UndoKind int

const (
	UndoDelete UndoKind = 0
	UndoInsert UndoKind = 1
)

const UndoHistoryMax = 64

type UndoRecord struct {
	Kind    UndoKind
	LineNum uint
	Offset  uint
	Text    []byte
	Len     uint
}

type UndoGroup struct {
	Buffer       *Buffer
	BufferSerial uint32
	GroupSerial  uint32
	Before       Location
	Records      []UndoRecord
	Count        uint16
}

// UndoHistory stores editor undo groups (Emacs-style command grouping).
// Call BeginCommand before edits in a command and EndCommand when it finishes;
// RecordEdit uses Pending.Before from BeginCommand to restore the cursor.
type UndoHistory struct {
	Groups          [UndoHistoryMax]UndoGroup
	Pending         UndoGroup
	NextGroupSerial uint32
	Count           uint8
	IsReplaying     bool
}

// UndoReplay provides editor callbacks used while replaying undo records.
type UndoReplay struct {
	InsertText     func(lineNumber, offset uint, text []byte) bool
	DeleteText     func(lineNumber, offset uint, text []byte) bool
	SetCursor      func(loc Location)
	SwitchBuffer   func(bp *Buffer)
	CurrentBuffer  func() *Buffer
	OnRestoredSave func(bp *Buffer)
}

func (group *UndoGroup) reset() {
	group.Records = nil
	group.Buffer = nil
	group.BufferSerial = 0
	group.GroupSerial = 0
	group.Before = Location{}
	group.Count = 0
}

func (h *UndoHistory) BeginCommand(bp *Buffer, before Location) {
	if h == nil || h.IsReplaying || bp == nil {
		return
	}
	h.Pending.reset()
	h.Pending.Buffer = bp
	h.Pending.BufferSerial = bp.Serial
	h.Pending.Before = before
}

func (h *UndoHistory) EndCommand() {
	if h == nil || h.IsReplaying {
		return
	}
	if h.Pending.Count == 0 {
		h.Pending.reset()
		return
	}
	if h.Count == UndoHistoryMax {
		h.Groups[h.Count-1].reset()
		h.Count--
	}
	for i := int(h.Count); i > 0; i-- {
		h.Groups[i] = h.Groups[i-1]
	}
	h.Pending.GroupSerial = h.NextGroupSerial
	h.NextGroupSerial++
	h.Groups[0] = h.Pending
	h.Pending = UndoGroup{}
	h.Count++
}

func (h *UndoHistory) ForgetBuffer(bp *Buffer) {
	if h == nil || bp == nil {
		return
	}
	for i := uint8(0); i < h.Count; {
		if h.Groups[i].Buffer == bp {
			h.Groups[i].reset()
			for j := i; j < h.Count-1; j++ {
				h.Groups[j] = h.Groups[j+1]
			}
			h.Count--
			h.Groups[h.Count].reset()
			continue
		}
		i++
	}
	if h.Pending.Buffer == bp {
		h.Pending.reset()
	}
}

func (h *UndoHistory) NoteBufferSaved(bp *Buffer) {
	if h == nil || bp == nil {
		return
	}
	if h.Count > 0 && h.Groups[0].Buffer == bp {
		bp.SavedUndoSerial = h.Groups[0].GroupSerial
	} else {
		bp.SavedUndoSerial = 0
	}
}

func (h *UndoHistory) appendRecordAt(bp *Buffer, before Location, lineNumber, offset uint, kind UndoKind, text []byte) bool {
	if h == nil || h.IsReplaying || len(text) == 0 {
		return true
	}
	if text == nil {
		return false
	}
	group := &h.Pending
	if group.Buffer == nil {
		group.Buffer = bp
		group.BufferSerial = bp.Serial
		group.Before = before
	}
	if group.Buffer != bp || group.BufferSerial != bp.Serial {
		return true
	}
	need := int(group.Count) + 1
	if need > cap(group.Records) {
		newCap := 16
		if c := cap(group.Records); c > 0 {
			newCap = c * 2
		}
		for newCap < need {
			newCap *= 2
		}
		newRecords := make([]UndoRecord, group.Count, newCap)
		copy(newRecords, group.Records)
		group.Records = newRecords
	}
	if int(group.Count) >= len(group.Records) {
		group.Records = group.Records[:group.Count+1]
	}
	record := &group.Records[group.Count]
	record.Kind = kind
	record.LineNum = lineNumber
	record.Offset = offset
	record.Len = uint(len(text))
	record.Text = append([]byte(nil), text...)
	group.Count++
	return true
}

// RecordEdit appends undo records for a buffer set-text operation.
func (h *UndoHistory) RecordEdit(bp *Buffer, before Location, begin Location, oldText, newText []byte) {
	if h == nil {
		return
	}
	if len(oldText) == len(newText) && len(oldText) > 0 && bytes.Equal(oldText, newText) {
		return
	}
	_ = h.appendRecordAt(bp, before, begin.Line, begin.Offset, UndoDelete, oldText)
	_ = h.appendRecordAt(bp, before, begin.Line, begin.Offset, UndoInsert, newText)
}

// Undo replays the most recent undo group using editor-provided callbacks.
func (h *UndoHistory) Undo(replay UndoReplay) bool {
	if h == nil || h.Count == 0 {
		return false
	}
	group := h.Groups[0]
	for j := 0; j < int(h.Count)-1; j++ {
		h.Groups[j] = h.Groups[j+1]
	}
	h.Groups[h.Count-1].reset()
	h.Count--

	if group.Buffer == nil || group.BufferSerial != group.Buffer.Serial {
		group.reset()
		return false
	}

	if replay.CurrentBuffer != nil && replay.SwitchBuffer != nil {
		if replay.CurrentBuffer() != group.Buffer {
			replay.SwitchBuffer(group.Buffer)
		}
	}

	h.IsReplaying = true
	defer func() { h.IsReplaying = false }()

	ok := true
	for j := uint16(group.Count); j > 0; j-- {
		record := &group.Records[j-1]
		if record.Kind == UndoInsert {
			if replay.DeleteText != nil {
				ok = replay.DeleteText(record.LineNum, record.Offset, record.Text)
			}
		} else if replay.InsertText != nil {
			ok = replay.InsertText(record.LineNum, record.Offset, record.Text)
		}
		if !ok {
			break
		}
	}
	if ok && replay.SetCursor != nil {
		replay.SetCursor(group.Before)
	}
	if ok && replay.OnRestoredSave != nil {
		gbp := group.Buffer
		topSerial := uint32(0)
		if h.Count > 0 && h.Groups[0].Buffer == gbp {
			topSerial = h.Groups[0].GroupSerial
		}
		if topSerial == gbp.SavedUndoSerial {
			replay.OnRestoredSave(gbp)
		}
	}
	group.reset()
	return ok
}
