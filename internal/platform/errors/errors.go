// Package errors provides standardized domain error types.
// Services return these sentinel errors; the error-mapping interceptor
// converts them to proper gRPC status codes for Envoy transcoding.
package errors

import "errors"

// Sentinel domain errors. Use errors.Is() to check.
var (
	ErrNotFound        = errors.New("not found")
	ErrAlreadyExists   = errors.New("already exists")
	ErrForbidden       = errors.New("forbidden")
	ErrInvalidInput    = errors.New("invalid input")
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrRateLimited     = errors.New("rate limited")
)
