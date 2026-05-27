// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrInstanceNotFound(t *testing.T) {
	if ErrInstanceNotFound == nil {
		t.Fatal("ErrInstanceNotFound should not be nil")
	}
	if ErrInstanceNotFound.Error() != "no credentials for instance" {
		t.Errorf("unexpected error message: %q", ErrInstanceNotFound.Error())
	}
}

func TestErrMethodNotSupported(t *testing.T) {
	if ErrMethodNotSupported == nil {
		t.Fatal("ErrMethodNotSupported should not be nil")
	}
	if ErrMethodNotSupported.Error() != "method not supported" {
		t.Errorf("unexpected error message: %q", ErrMethodNotSupported.Error())
	}
}

func TestErrInstanceNotFound_Wrapping(t *testing.T) {
	wrapped := fmt.Errorf("instance 12345: %w", ErrInstanceNotFound)
	if !errors.Is(wrapped, ErrInstanceNotFound) {
		t.Error("wrapped error should match ErrInstanceNotFound via errors.Is")
	}
}

func TestErrMethodNotSupported_Wrapping(t *testing.T) {
	wrapped := fmt.Errorf("customMethod: %w", ErrMethodNotSupported)
	if !errors.Is(wrapped, ErrMethodNotSupported) {
		t.Error("wrapped error should match ErrMethodNotSupported via errors.Is")
	}
}

func TestDomainErrors_AreDistinct(t *testing.T) {
	if errors.Is(ErrInstanceNotFound, ErrMethodNotSupported) {
		t.Error("ErrInstanceNotFound and ErrMethodNotSupported should be distinct")
	}
}
