// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

type MockProcessor struct {
	Err error
}

func (pc *MockProcessor) Type() string {
	return "mock"
}

func (pc *MockProcessor) Handle(event File) error {
	return pc.Err
}
