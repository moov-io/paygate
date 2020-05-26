// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"context"
)

type MockPublisher struct {
	Xfers   map[string]Xfer
	Cancels map[string]CanceledTransfer

	Err error
}

func NewMockPublisher() *MockPublisher {
	return &MockPublisher{
		Xfers:   make(map[string]Xfer),
		Cancels: make(map[string]CanceledTransfer),
	}
}

func (p *MockPublisher) Upload(xfer Xfer) error {
	p.Xfers[xfer.Transfer.TransferID] = xfer
	return p.Err
}

func (p *MockPublisher) Cancel(msg CanceledTransfer) error {
	p.Cancels[msg.TransferID] = msg
	return p.Err
}

func (p *MockPublisher) Shutdown(ctx context.Context) {}
