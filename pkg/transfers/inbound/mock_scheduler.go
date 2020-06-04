// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

type MockScheduler struct {
	Err error
}

func (s *MockScheduler) Start() error {
	return s.Err
}

func (s *MockScheduler) Shutdown() {}
