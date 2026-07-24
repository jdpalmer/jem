package buffer

// Process-wide defaults applied by Create. Runtime updates these when
// editor variables change (no create hook).
var (
	DefaultFillCol           = 80
	DefaultIndent            = IndentConfig{Width: 2, Continued: 4}
	DefaultWhitespaceCleanup bool
)
