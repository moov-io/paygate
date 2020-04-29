// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package fundflow

import (
	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/client"
)

type MockStrategy struct {
	Files []*ach.File
	Err   error
}

func (s *MockStrategy) Originate(xfer *client.Transfer, source Source, destination Destination) ([]*ach.File, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	return s.Files, nil
}

func (s *MockStrategy) HandleReturn(returned *ach.File, xfer *client.Transfer) ([]*ach.File, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	return s.Files, nil
}
