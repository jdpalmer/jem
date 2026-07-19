package model

import "github.com/jdpalmer/jem/event"

// TakeMacroPromptReply consumes the next PromptReplyEvent while a macro is playing.
// playing is false when not in macro playback; otherwise text/pr are the canned reply.
func TakeMacroPromptReply() (text string, pr PromptResult, playing bool) {
	if !State.IsPlaying() {
		return "", 0, false
	}
	if State.PlayPos >= len(State.Macro) {
		return "", PromptResultNo, true
	}
	ev, isReply := State.Macro[State.PlayPos].(event.PromptReplyEvent)
	if !isReply {
		return "", PromptResultNo, true
	}
	State.PlayPos++
	if ev.Text == "" {
		return "", PromptResultNo, true
	}
	return ev.Text, PromptResultYes, true
}
