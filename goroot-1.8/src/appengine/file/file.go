// Copyright 2012 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Package file provides a client for Google Cloud Storage.
//
// NOTE: The Files API was deprecated on June 11, 2013 (v1.8.1) and was shut
// down on September 9, 2015, at which point these functions stopped working.
// Use Google Cloud Storage instead (https://cloud.google.com/storage/).
package file

import (
	"errors"
	"fmt"
	"os"

	"appengine"
	aipb "appengine_internal/app_identity"
	"appengine_internal"
	blobpb "appengine_internal/blobstore"
)

var errDeprecated = errors.New("deprecated: use the Google Cloud Storage API instead; see google.golang.org/cloud/storage")

// Stat stats a file.
func Stat(c appengine.Context, filename string) (os.FileInfo, error) {
	return nil, errDeprecated
}

// DefaultBucketName returns the name of this application's
// default Google Cloud Storage bucket.
func DefaultBucketName(c appengine.Context) (string, error) {
	req := &aipb.GetDefaultGcsBucketNameRequest{}
	res := &aipb.GetDefaultGcsBucketNameResponse{}

	err := c.Call("app_identity_service", "GetDefaultGcsBucketName", req, res, nil)
	if err != nil {
		return "", fmt.Errorf("file: no default bucket name returned in RPC response: %v", res)
	}
	return res.GetDefaultGcsBucketName(), nil
}

// Delete deletes a file.
func Delete(c appengine.Context, filename string) error {
	return errDeprecated
}

func init() {
	appengine_internal.RegisterErrorCodeMap("blobstore", blobpb.BlobstoreServiceError_ErrorCode_name)
}
