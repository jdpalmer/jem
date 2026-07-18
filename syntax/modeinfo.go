package syntax

import "github.com/jdpalmer/jem/buffer"

// Palette supplies theme-dependent syntax styles.
type Palette struct {
	NormalStyle  buffer.TextStyle
	CommentStyle buffer.TextStyle
}

// PackagePalette is set by the editor during init.
var PackagePalette Palette
