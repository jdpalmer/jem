package term

import (
	"bufio"
	"fmt"
	"os"
	"time"
	"unicode/utf8"
)

const pasteMaxBytes = 65536

// termReadKeyUnlocked reads and decodes one key. Caller must hold termReadMu.
func termReadKeyUnlocked() (uint32, bool) {
	for {
		var buf [1]byte
		_, err := termReader.Read(buf[:])
		if err != nil {
			return 0, false
		}

		firstByte := buf[0]

		if firstByte >= 0x80 {
			k := termDecodeUTF8KeyFromFirstByte(firstByte)
			return k, k != 0
		}

		if firstByte != 0x1B {
			return uint32(firstByte), true
		}

		_, err = termReader.Read(buf[:])
		if err != nil {
			return 0x1B, true
		}
		secondByte := buf[0]

		if secondByte == '[' {
			if key := termDecodeCSISequence(); key != 0 {
				return key, true
			}
			continue
		}

		if secondByte == 'O' {
			if key := termDecodeSS3Sequence(); key != 0 {
				return key, true
			}
			continue
		}

		if secondByte >= 0x50 && secondByte <= 0x5D || secondByte == '^' || secondByte == '_' {
			termDiscardControlString()
			continue
		}

		// Meta (Alt) prefix: ESC followed by a printable key. Return ESC now and
		// leave the printable byte in the buffer for the next read.
		if secondByte >= 0x20 && secondByte < 0x7F {
			_ = termReader.UnreadByte()
			return 0x1B, true
		}

		termDiscardEscapeSequenceTail(int(secondByte))
	}
}

// termReadKeyImpl is the test entry point; it acquires termReadMu.
func termReadKeyImpl() (uint32, bool) {
	termReadMu.Lock()
	defer termReadMu.Unlock()
	return termReadKeyUnlocked()
}

// ReadKey reads and decodes one key event from the terminal.
func ReadKey() (uint32, bool) {
	termReadMu.Lock()
	defer termReadMu.Unlock()
	k, ok := termReadKeyUnlocked()
	return k, ok
}

// TryReadKey reads one key with a timeout, polling the fd directly.
func TryReadKey(timeout time.Duration) (uint32, bool) {
	termReadMu.Lock()
	defer termReadMu.Unlock()
	if termReader.Buffered() == 0 && !termWaitReadable(termFd, timeout) {
		return 0, false
	}
	return termReadKeyUnlocked()
}

// ResetReader clears read state after handing stdin back from a shell subjob.
func ResetReader() {
	termReadMu.Lock()
	defer termReadMu.Unlock()
	_ = termFile.SetReadDeadline(time.Time{})
	termReader = bufio.NewReader(termFile)
}

func csiModifierFlags(modifierParam int) uint32 {
	modifierBits := 0
	if modifierParam > 0 {
		modifierBits = modifierParam - 1
	}
	var flags uint32
	if modifierBits&1 != 0 {
		flags |= SHIFT
	}
	if modifierBits&2 != 0 {
		flags |= META
	}
	if modifierBits&4 != 0 {
		flags |= CTL
	}
	return flags
}

func cursorKeyCode(finalByte int, modifiers uint32) (uint32, bool) {
	var base uint32
	switch finalByte {
	case 'A':
		base = KeyUp
	case 'B':
		base = KeyDown
	case 'C':
		base = KeyRight
	case 'D':
		base = KeyLeft
	case 'H':
		base = KeyHome
	case 'F':
		base = KeyEnd
	default:
		return 0, false
	}
	return base | modifiers, true
}

func tildeKeyCode(tildeParam int, modifiers uint32) (uint32, bool) {
	switch tildeParam {
	case 1, 7:
		return cursorKeyCode('H', modifiers)
	case 2:
		return 0, false
	case 3:
		return KeyDelete | modifiers, true
	case 4, 8:
		return cursorKeyCode('F', modifiers)
	case 5:
		return KeyPageUp | modifiers, true
	case 6:
		return KeyPageDown | modifiers, true
	default:
		return 0, false
	}
}

func termDecodeSS3Sequence() uint32 {
	var buf [1]byte
	_, err := termReader.Read(buf[:])
	if err != nil {
		return 0
	}
	key, ok := cursorKeyCode(int(buf[0]), 0)
	if !ok {
		return 0
	}
	return key
}

func termDiscardEscapeSequenceTail(secondByte int) {
	if secondByte >= finalByteMin && secondByte <= finalByteMax {
		return
	}

	for i := 0; i < escapeSequenceMaxBytes; i++ {
		var buf [1]byte
		_, err := termReader.Read(buf[:])
		if err != nil {
			return
		}
		if buf[0] >= finalByteMin && buf[0] <= finalByteMax {
			return
		}
	}
}

func termDiscardControlString() {
	for i := 0; i < escapeSequenceMaxBytes; i++ {
		var buf [1]byte
		_, err := termReader.Read(buf[:])
		if err != nil {
			return
		}

		if buf[0] == 0x07 {
			return
		}
		if buf[0] == '\\' {
			return
		}
		if buf[0] == 0x9D || buf[0] == 0x9E || buf[0] == 0x9F {
			return
		}
	}
}

func termDecodeSGRMouse(params [3]int, finalByte int, out *uint32) bool {
	if finalByte == 'm' {
		return false
	}
	buttonCode := params[0]
	col := params[1] - 1
	row := params[2] - 1
	termStoreMousePosition(col, row)

	if buttonCode == mouseButtonWheelUp {
		*out = MouseWheelUp
		return true
	}
	if buttonCode == mouseButtonWheelDown {
		*out = MouseWheelDown
		return true
	}
	if buttonCode == mouseButtonDrag {
		*out = MouseDrag
		return true
	}
	if buttonCode == mouseButtonLeft {
		*out = MouseLeft
		return true
	}
	return false
}

func termStoreMousePosition(col, row int) {
	if col < 0 {
		col = 0
	}
	if row < 0 {
		row = 0
	}
	lastMouseCol = col
	lastMouseRow = row
	if PackageHooks.OnMouse != nil {
		PackageHooks.OnMouse(col, row)
	}
}

func csiUKeyCode(codepoint int, modifierParam int, eventType int) uint32 {

	if eventType == keyEventRelease {
		return 0
	}

	res := uint32(0)
	modifiers := uint32(0)
	if modifierParam > 0 {
		modifiers = csiModifierFlags(modifierParam)
	}

	switch codepoint {
	case codepointEscape:
		res = uint32(codepointEscape) | modifiers
		goto out
	case codepointEnter:
		if modifiers&CTL != 0 && modifiers&SHIFT != 0 {
			res = CTL | SHIFT | 0x0D
			goto out
		}
		if modifiers&SHIFT != 0 {
			res = SHIFT | 0x0D
			goto out
		}
		res = 0x0D
		goto out
	case codepointLinefeed:
		res = CTL | 'J'
		goto out
	case codepointTab:
		if modifiers&CTL != 0 && modifiers&SHIFT == 0 && modifiers&META == 0 {
			res = CTL | KeyTab
			goto out
		}
		if modifiers&SHIFT != 0 && modifiers&CTL == 0 && modifiers&META == 0 {
			res = SHIFT | KeyTab
			goto out
		}
		res = KeyTab
		goto out
	case codepointBackspace:
		if modifiers&CTL != 0 && modifiers&META == 0 {
			res = META | 'H'
			goto out
		}
		res = CTL | 'H'
		goto out
	case codepointSpace:
		if modifiers&CTL != 0 && modifiers&META == 0 {
			res = CTL | ' '
			goto out
		}
		if modifiers&SHIFT != 0 && modifiers&CTL == 0 && modifiers&META == 0 {
			res = ' '
			goto out
		}
		res = uint32(codepointSpace) | modifiers
		goto out
	}

	switch codepoint {
	case kittyKeyLeft:
		res = KeyLeft | modifiers
		goto out
	case kittyKeyUp:
		res = KeyUp | modifiers
		goto out
	case kittyKeyDown:
		res = KeyDown | modifiers
		goto out
	case kittyKeyRight:
		res = KeyRight | modifiers
		goto out
	case kittyKeyHome:
		res = KeyHome | modifiers
		goto out
	case kittyKeyEnd:
		res = KeyEnd | modifiers
		goto out
	case kittyKeyPageUp:
		res = KeyPageUp | modifiers
		goto out
	case kittyKeyPageDown:
		res = KeyPageDown | modifiers
		goto out
	case kittyKeyDelete:
		res = KeyDelete | modifiers
		goto out
	}

	if codepoint >= codepointSpace && codepoint < asciiByteLimit {
		modifiers &= ^SHIFT
		if modifiers&CTL != 0 && modifiers&META == 0 {
			if codepoint >= 'a' && codepoint <= 'z' {
				codepoint -= ('a' - 'A')
			}
			res = CTL | uint32(codepoint)
			goto out
		}
		if modifiers&META != 0 {
			res = uint32(codepoint) | META
			goto out
		}
		res = uint32(codepoint)
		goto out
	}
	if codepoint >= asciiByteLimit && codepoint < UnicodeLimit {
		modifiers &= ^SHIFT
		if modifiers&CTL != 0 && modifiers&META == 0 && codepoint >= 'a' && codepoint <= 'z' {
			codepoint -= ('a' - 'A')
			res = uint32(codepoint & 0x1F)
			goto out
		}
		if modifiers&META != 0 {
			res = uint32(codepoint) | META
			goto out
		}
		res = uint32(codepoint) | modifiers
		goto out
	}

out:
	return res
}

func termDecodeUTF8KeyFromFirstByte(firstByte byte) uint32 {
	var buf [4]byte
	n := 1
	buf[0] = firstByte
	for n < 4 {
		_, err := termReader.Read(buf[n : n+1])
		if err != nil {
			return 0
		}
		if buf[n] < 0x80 || buf[n] > 0xBF {
			_ = termReader.UnreadByte()
			break
		}
		n++
	}

	r, _ := utf8.DecodeRune(buf[:n])
	if r == utf8.RuneError {
		return 0
	}
	return uint32(r)
}

func termDecodeCSISequence() uint32 {

	var payload [64]byte
	payloadLength := 0
	finalByte := 0
	var params [3]int
	var subparams [3]int
	paramIndex := 0
	parsingSubparam := false

	for {
		var buf [1]byte
		_, err := termReader.Read(buf[:])
		if err != nil {
			return 0
		}
		b := buf[0]
		if payloadLength < len(payload)-1 {
			payload[payloadLength] = b
			payloadLength++
		}
		if b >= finalByteMin && b <= finalByteMax {
			finalByte = int(b)
			break
		}
	}

	if finalByte == '~' && payloadLength > 0 {
		firstParam := 0
		for i := 0; i < payloadLength && payload[i] >= '0' && payload[i] <= '9'; i++ {
			firstParam = firstParam*10 + int(payload[i]-'0')
		}
		if firstParam == 200 {
			termHandleBracketedPaste()
			return KeyPasteComplete
		}
	}

	for i := 0; i < payloadLength && paramIndex < 3; i++ {
		c := payload[i]
		if c == ';' {
			paramIndex++
			parsingSubparam = false
			continue
		}
		if c == ':' {
			parsingSubparam = true
			continue
		}
		if c < '0' || c > '9' {
			continue
		}
		if !parsingSubparam {
			params[paramIndex] = params[paramIndex]*10 + int(c-'0')
		} else {
			subparams[paramIndex] = subparams[paramIndex]*10 + int(c-'0')
		}
	}

	if (finalByte == 'M' || finalByte == 'm') && payload[0] == '<' {
		var key uint32
		if termDecodeSGRMouse(params, finalByte, &key) {
			return key
		}
		return 0
	}

	if finalByte == 'u' {
		codepoint := params[0]
		modifierParam := params[1]
		if modifierParam == 0 {
			modifierParam = defaultModifierParam
		}
		eventType := subparams[1]
		if eventType == 0 {
			eventType = keyEventPress
		}
		return csiUKeyCode(codepoint, modifierParam, eventType)
	}

	if finalByte == '~' {
		if params[0] == codepointEscape && params[1] > 0 && params[2] > 0 {
			return csiUKeyCode(params[2], params[1], keyEventPress)
		}
		modifiers := uint32(0)
		if params[1] > 0 {
			modifiers = csiModifierFlags(params[1])
		}
		key, ok := tildeKeyCode(params[0], modifiers)
		if ok {
			return key
		}
		return 0
	}

	if finalByte == 'Z' {
		return SHIFT | KeyTab
	}

	if finalByte >= 'A' && finalByte <= 'Z' {
		modifiers := uint32(0)
		if params[0] == 1 && params[1] > 0 {
			modifiers = csiModifierFlags(params[1])
		} else if params[0] > 1 && params[1] == 0 {
			modifiers = csiModifierFlags(params[0])
		}
		key, ok := cursorKeyCode(finalByte, modifiers)
		if ok {
			return key
		}
		return 0
	}

	return 0
}

func pasteTerminatorAtTail(data []byte) bool {
	if len(data) < 6 {
		return false
	}
	i := len(data) - 6
	return data[i] == 0x1B && data[i+1] == '[' &&
		data[i+2] == '2' && data[i+3] == '0' &&
		data[i+4] == '1' && data[i+5] == '~'
}

func termDeliverPaste(data []byte) {
	if len(data) == 0 || PackageHooks.OnPaste == nil {
		return
	}
	paste := append([]byte(nil), data...)
	PackageHooks.OnPaste(paste)
}

// termHandleBracketedPaste reads paste payload until CSI 201~.
func termHandleBracketedPaste() {
	var buf []byte
	var recent [6]byte
	recentLen := 0
	truncated := false

	updateRecent := func(b byte) {
		if recentLen < 6 {
			recent[recentLen] = b
			recentLen++
			return
		}
		copy(recent[:], recent[1:])
		recent[5] = b
	}

	for {
		var b [1]byte
		_, err := termReader.Read(b[:])
		if err != nil {
			return
		}
		updateRecent(b[0])

		if !truncated {
			buf = append(buf, b[0])
			if len(buf) >= 6 && pasteTerminatorAtTail(buf) {
				termDeliverPaste(buf[:len(buf)-6])
				return
			}
			if len(buf) > pasteMaxBytes {
				truncated = true
				fmt.Fprintln(os.Stderr, "jem: paste exceeds 64 KiB; truncating")
				termDeliverPaste(buf[:pasteMaxBytes])
			}
			continue
		}

		if recentLen == 6 && pasteTerminatorAtTail(recent[:]) {
			return
		}
	}
}
