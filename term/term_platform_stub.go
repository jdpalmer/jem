//go:build !windows

package term

func termPlatformInitConsole() error { return nil }
func termPlatformCloseConsole()      {}
func termPlatformAfterFlush()        {}
