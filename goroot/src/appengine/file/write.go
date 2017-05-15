// Copyright 2012 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package file

import (
	"io"

	"appengine"
)

// CreateOptions are the file creation options.
// A nil *CreateOptions means the same as using all the default values.
type CreateOptions struct {
	// MIMEType is the MIME type to use.
	// The empty string means to use "application/octet-stream".
	MIMEType string

	// The Google Cloud Storage bucket name to use.
	// The empty string means to use the default bucket.
	BucketName string
}

// Create creates a new file, opened for append.
//
// The file must be closed when done writing.
//
// The provided filename may be absolute ("/gs/bucketname/objectname")
// or may be just the filename, in which case the bucket is determined
// from opts.  The absolute filename is returned.
func Create(c appengine.Context, filename string, opts *CreateOptions) (wc io.WriteCloser, absFilename string, err error) {
	return nil, "", errDeprecated
}

// writer is used for writing blobs. Blobs aren't fully written until
// Close is called, at which point the key can be retrieved by calling
// the Key method.
type writer struct {
}

func (w *writer) Write(p []byte) (n int, err error) {
	return 0, errDeprecated
}

func (w *writer) Close() error {
	return errDeprecated
}
