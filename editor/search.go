package editor

import "github.com/jdpalmer/jem/search"

func CmdSearchForward(f bool, n int) bool  { _ = f; _ = n; return search.SearchForward() }
func CmdSearchBackward(f bool, n int) bool { _ = f; _ = n; return search.SearchBackward() }
func CmdIsearchForward(f bool, n int) bool { _ = f; _ = n; return search.IsearchForward() }
func CmdIsearchBackward(f bool, n int) bool {
	_ = f
	_ = n
	return search.IsearchBackward()
}
func CmdIsearchReForward(f bool, n int) bool { _ = f; _ = n; return search.IsearchReForward() }
func CmdIsearchReBackward(f bool, n int) bool {
	_ = f
	_ = n
	return search.IsearchReBackward()
}
func CmdToggleSearchScope(f bool, n int) bool { _ = f; _ = n; return search.ToggleSearchScope() }
func CmdQueryReplace(f bool, n int) bool      { _ = f; _ = n; return search.QueryReplace() }
func CmdQueryReReplace(f bool, n int) bool    { _ = f; _ = n; return search.QueryReReplace() }
