package mode

// IndentBytesForColForTest exports indentBytesForCol for unit tests.
func IndentBytesForColForTest(col int) []byte {
	return indentBytesForCol(col)
}
