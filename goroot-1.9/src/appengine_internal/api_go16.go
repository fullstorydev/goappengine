// +build !go1.8

package appengine_internal

import (
	"log"
	"net/http"
	"sync"
)

var (
	ctxsMu sync.Mutex
	ctxs   = make(map[*http.Request]context)
)

func NewContext(r *http.Request) context {
	ctxsMu.Lock()
	defer ctxsMu.Unlock()

	c := ctxs[r]
	if c == nil {
		// Someone passed in an http.Request that is not in-flight.
		// We panic here rather than panicking at a later point
		// so that backtraces will be more sensible.
		log.Panic("appengine: NewContext passed an unknown http.Request")
	}
	return c
}

func registerContext(r *http.Request, c context) *http.Request {
	ctxsMu.Lock()
	ctxs[r] = c
	ctxsMu.Unlock()
	return r
}

func registerTestContext(r *http.Request, c context) *http.Request {
	ctxsMu.Lock()
	defer ctxsMu.Unlock()
	if _, ok := ctxs[r]; ok {
		log.Panic("req already associated with context")
	}
	ctxs[r] = c
	return r
}

func unregisterContext(r *http.Request) {
	ctxsMu.Lock()
	delete(ctxs, r)
	ctxsMu.Unlock()
}
