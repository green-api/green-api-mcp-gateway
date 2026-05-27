// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package domain

import "errors"

var (
	// ErrInstanceNotFound indicates that the requested instance ID is not registered.
	ErrInstanceNotFound = errors.New("no credentials for instance")

	// ErrMethodNotSupported indicates an unsupported API method.
	ErrMethodNotSupported = errors.New("method not supported")
)
