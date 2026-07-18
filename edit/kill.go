package edit

import "github.com/jdpalmer/jem/app"

var killRing [16][]byte
var killRingCount uint8
var killRingIdx uint8
var killAggregate []byte

// KillBegin starts or continues a kill sequence (aggregates successive kills).
func KillBegin() {
	if app.State.KillState == app.CmdStateNone {
		killAggregate = nil
	}
	app.State.KillState = app.CmdStateCurrent
}

// KillAppend adds text to the kill ring and aggregate.
func KillAppend(text []byte) bool {
	if len(text) == 0 {
		return true
	}
	killAggregate = append(killAggregate, text...)
	entry := append([]byte(nil), text...)
	killRing[killRingIdx] = entry
	killRingIdx = (killRingIdx + 1) % 16
	if killRingCount < 16 {
		killRingCount++
	}
	return true
}

// KillBytes returns the current kill aggregate.
func KillBytes() []byte {
	return killAggregate
}

// KillWriteClipboard copies the kill aggregate (or last ring entry) to the clipboard.
func KillWriteClipboard() {
	if len(killAggregate) == 0 && killRingCount > 0 {
		idx := (killRingIdx + 15) % 16
		_ = ClipboardWrite(killRing[idx])
		return
	}
	if len(killAggregate) > 0 {
		_ = ClipboardWrite(killAggregate)
	}
}

// KillReadClipboard replaces the kill aggregate with the system clipboard contents.
// Returns false when the clipboard is unavailable or empty.
func KillReadClipboard() bool {
	data, ok := ClipboardRead()
	if !ok {
		return false
	}
	killAggregate = make([]byte, len(data))
	copy(killAggregate, data)
	entry := make([]byte, len(data))
	copy(entry, data)
	killRing[killRingIdx] = entry
	killRingIdx = (killRingIdx + 1) % 16
	if killRingCount < 16 {
		killRingCount++
	}
	return true
}

// ResetKillForTests clears kill-ring state between tests.
func ResetKillForTests() {
	killAggregate = nil
	killRingCount = 0
	killRingIdx = 0
	for i := range killRing {
		killRing[i] = nil
	}
}
