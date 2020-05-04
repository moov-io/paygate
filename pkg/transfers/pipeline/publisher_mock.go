// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

type MockPublisher struct {
	Err error
}

func (p *MockPublisher) Upload(xfer Xfer) error {
	return p.Err
}

func (p *MockPublisher) Cancel(xfer Xfer) error {
	return p.Err
}

// type MockConsumer struct {
// 	Err error
// }

// func (c *MockConsumer) HandleUpload(xfer Xfer) error {
// 	return c.Err
// }

// func (c *MockConsumer) HandleCancel(xfer Xfer) error {
// 	return c.Err
// }
