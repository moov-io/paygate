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

func (strat *MockStrategy) Originate(xfer *client.Transfer, source Source, destination Destination) ([]*ach.File, error) {
	if strat.Err != nil {
		return nil, strat.Err
	}
	return strat.Files, nil
}

func (strat *MockStrategy) HandleReturn(returned *ach.File, xfer *client.Transfer) ([]*ach.File, error) {
	if strat.Err != nil {
		return nil, strat.Err
	}
	return strat.Files, nil
}
