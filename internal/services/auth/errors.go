package auth

import "errors"

// ErrGenAccessToken is returned when we cannot create a JWT.
var ErrGenAccessToken = errors.New("failed to generate access token")

// ErrGenRefreshToken is returned when we cannot create a refresh token.
var ErrGenRefreshToken = errors.New("failed to generate refresh token")

// ErrRefreshTokens is returned when token refresh process fails.
var ErrRefreshTokens = errors.New("failed to refresh tokens")

// ErrSignOut is returned when sign out process fails.
var ErrSignOut = errors.New("failed to sign out")

// ErrSignOutAll is returned when sign out all process fails.
var ErrSignOutAll = errors.New("failed to sign out all devices")
