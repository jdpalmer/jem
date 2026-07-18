package app

func (a *AppState) IsRecording() bool { return a.RecordPos >= 0 }
func (a *AppState) IsPlaying() bool   { return a.PlayPos >= 0 }
