// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"context"
)

type MockPublisher struct {
	Err error
}

func (p *MockPublisher) Upload(xfer Xfer) error {
	return p.Err
}

func (p *MockPublisher) Cancel(xfer Xfer) error {
	return p.Err
}

func (p *MockPublisher) Shutdown(ctx context.Context) {}
