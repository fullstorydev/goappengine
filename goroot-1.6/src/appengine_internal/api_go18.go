// +build go1.8

package appengine_internal

import (
	stdcontext "context"
	"log"
	"net/http"
)

type keyType string

// ContextKey holds an App Engine context.
//
// It is exported so that google.golang.org/appengine/internal can share the
// same context key, allowing lookups to the App Engine context directly.
const ContextKey keyType = "App Engine context"

func NewContext(req *http.Request) context {
	c := req.Context().Value(ContextKey)
	if c == nil {
		// Someone passed in an http.Request that didn't come from
		// our server.
		// We panic here rather than panicking at a later point
		// so that backtraces will be more sensible.
		log.Panic("appengine: NewContext passed an unknown http.Request")
	}
	return c.(context)
}

func registerContext(r *http.Request, c context) *http.Request {
	ctx := r.Context()
	ctx = stdcontext.WithValue(ctx, ContextKey, c)
	return r.WithContext(ctx)
}

func registerTestContext(r *http.Request, c context) *http.Request {
	return registerContext(r, c)
}

func unregisterContext(r *http.Request) {
}
