package mode

// PendingMessage is set by Message and drained by display on the next update.
var PendingMessage string

// Message queues a minibuffer echo for display (avoids importing display).
func Message(msg string) {
	PendingMessage = msg
}

// TakeMessage returns and clears PendingMessage.
func TakeMessage() string {
	msg := PendingMessage
	PendingMessage = ""
	return msg
}
