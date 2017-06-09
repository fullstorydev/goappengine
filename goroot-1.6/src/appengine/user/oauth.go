// Copyright 2012 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package user

import (
	"errors"
	"appengine"

	pb "appengine_internal/user"
)

// CurrentOAuth returns the user associated with the OAuth consumer making this
// request. If the OAuth consumer did not make a valid OAuth request, or the
// scope is non-empty and the current user does not have this scope, this method
// will return an error.
func CurrentOAuth(c appengine.Context, scope string) (*User, error) {
	req := &pb.GetOAuthUserRequest{}
	if scope != "" {
		req.Scope = &scope
	}
	res := &pb.GetOAuthUserResponse{}

	err := c.Call("user", "GetOAuthUser", req, res, nil)
	if err != nil {
		return nil, err
	}
	return &User{
		Email:      *res.Email,
		AuthDomain: *res.AuthDomain,
		Admin:      res.GetIsAdmin(),
		ID:         *res.UserId,
		ClientID:   res.GetClientId(),
	}, nil
}

// OAuthConsumerKey is no longer supported. Use the golang.org/x/oauth2 packages
// directly for OAuth functionality.
func OAuthConsumerKey(c appengine.Context) (string, error) {
	return "", errors.New("user: OAuthConsumerKey is no longer supported")
}
