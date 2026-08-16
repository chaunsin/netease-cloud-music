// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"errors"
	"fmt"
)

// ErrClientClosed is returned when the Cookie transport rejects a request after Close.
var ErrClientClosed = errors.New("api client is closed")

// APIError reports a failure while handling an API response. StatusCode is the
// received HTTP status, and Err contains a response-processing error when one
// occurred.
type APIError struct {
	StatusCode int
	Err        error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("http status code: %d: %v", e.StatusCode, e.Err)
	}

	return fmt.Sprintf("http status code: %d", e.StatusCode)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

func newAPIError(statusCode int, err error) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Err:        err,
	}
}
