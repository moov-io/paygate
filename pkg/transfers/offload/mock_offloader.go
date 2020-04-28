// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package offload

type MockOffloader struct {
	Err error
}

func (off *MockOffloader) Upload(xfer Xfer) error {
	return off.Err
}

func (off *MockOffloader) Cancel(xfer Xfer) error {
	return off.Err
}
