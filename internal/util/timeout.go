// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package util

import (
	"errors"
	"time"
)

var (
	ErrTimeout = errors.New("timeout exceeded")
)

// Timeout will attempt to call f, but only for as long as t. If the function is still
// processing after t has elapsed then ErrTimeout will be returned.
func Timeout(f func() error, t time.Duration) error {
	answer := make(chan error)
	go func() {
		answer <- f()
	}()
	select {
	case err := <-answer:
		return err
	case <-time.After(t):
		return ErrTimeout
	}
}
