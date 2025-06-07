package auth

import "errors"

// ErrGenAccessToken is returned when we cannot create a JWT.
var ErrGenAccessToken = errors.New("failed to generate access token")
