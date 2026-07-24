package search

// KeySession is a multi-key modal driven by the editor listener stack
// (isearch, query-replace confirm). Open returns true when the session
// finishes without waiting for keys.
type KeySession interface {
	Open() (done bool)
	HandleKey(k uint32) (done bool)
	Close()
}
