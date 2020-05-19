// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"github.com/moov-io/ach"
)

type MockXferMerging struct {
	LatestXfer   *Xfer
	LatestCancel *CanceledTransfer

	Err error
}

func (merge *MockXferMerging) HandleXfer(xfer Xfer) error {
	merge.LatestXfer = &xfer
	return merge.Err
}

func (merge *MockXferMerging) HandleCancel(cancel CanceledTransfer) error {
	merge.LatestCancel = &cancel
	return merge.Err
}

func (merge *MockXferMerging) WithEachMerged(func(*ach.File) error) error {
	return merge.Err
}
