package tools

type GitLineDiff int

const (
	GitLineDiffNone GitLineDiff = iota
	GitLineDiffAdded
	GitLineDiffModified
	GitLineDiffDeleted
)
