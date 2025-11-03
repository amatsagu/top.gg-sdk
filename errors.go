package dbl

import "errors"

var (
	ErrRequestFailed         = errors.New("remote request failed with non 200 status code")
	ErrLocalRatelimit        = errors.New("exceeded local rate limit")
	ErrRemoteRatelimit       = errors.New("exceeded remote rate limit")
	ErrUnauthorizedRequest   = errors.New("unauthorized request")
	ErrRequireAuthentication = errors.New("endpoint requires valid token")
)
