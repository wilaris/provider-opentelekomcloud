package util

import (
	"errors"

	golangsdk "github.com/opentelekomcloud/gophertelekomcloud"
)

// IsNotFound returns true if the error represents an HTTP 404 response
// from GopherTelekomCloud, either as a typed ErrDefault404 or an
// ErrUnexpectedResponseCode with status 404.
func IsNotFound(err error) bool {
	var e404 golangsdk.ErrDefault404
	if errors.As(err, &e404) {
		return true
	}
	var euc golangsdk.ErrUnexpectedResponseCode
	return errors.As(err, &euc) && euc.Actual == 404
}

// IsConflict returns true if the error represents an HTTP 409 response
// from GopherTelekomCloud, either as a typed ErrDefault409 or an
// ErrUnexpectedResponseCode with status 409.
func IsConflict(err error) bool {
	var e409 golangsdk.ErrDefault409
	if errors.As(err, &e409) {
		return true
	}
	var euc golangsdk.ErrUnexpectedResponseCode
	return errors.As(err, &euc) && euc.Actual == 409
}

// IsBadRequest returns true if the error represents an HTTP 400 response
// from GopherTelekomCloud, either as a typed ErrDefault400 or an
// ErrUnexpectedResponseCode with status 400.
func IsBadRequest(err error) bool {
	var e400 golangsdk.ErrDefault400
	if errors.As(err, &e400) {
		return true
	}
	var euc golangsdk.ErrUnexpectedResponseCode
	return errors.As(err, &euc) && euc.Actual == 400
}

// IsServerError returns true if the error represents an HTTP 500 response
// from GopherTelekomCloud, either as a typed ErrDefault500 or an
// ErrUnexpectedResponseCode with status 500.
func IsServerError(err error) bool {
	var e500 golangsdk.ErrDefault500
	if errors.As(err, &e500) {
		return true
	}
	var euc golangsdk.ErrUnexpectedResponseCode
	return errors.As(err, &euc) && euc.Actual == 500
}
