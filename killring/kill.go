package killring

// killSeq tracks chained kill sequences across command ticks:
// 0 = idle, 1 = chained from previous tick, 2 = active this tick.
var killSeq int

var killRing [16][]byte
var killRingCount uint8
var killRingIdx uint8
var killAggregate []byte

// InSequence reports whether a kill sequence is active or chained.
func InSequence() bool {
	return killSeq != 0
}

// ClearSequence resets kill-sequence chaining (e.g. on yank from idle).
func ClearSequence() {
	killSeq = 0
}

// KillBegin starts or continues a kill sequence (aggregates successive kills).
func KillBegin() {
	if killSeq == 0 {
		killAggregate = nil
	}
	killSeq = 2
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

// Tick decays kill-sequence state at the end of a command tick.
func Tick() {
	if killSeq != 0 {
		killSeq--
	}
}

// ResetForTests clears kill-ring state between tests.
func ResetForTests() {
	killAggregate = nil
	killRingCount = 0
	killRingIdx = 0
	killSeq = 0
	for i := range killRing {
		killRing[i] = nil
	}
}
