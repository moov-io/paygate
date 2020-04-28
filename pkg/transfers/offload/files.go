// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package offload

import (
	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
)

// Files attempts to upload all files to the Offloader and returns
// all errors as a base.ErrorList.
//
// All files are attempted to be offloaded as downstream processors
// are expected to de-duplicate files.
func Files(off Offloader, xfer *client.Transfer, files []*ach.File) error {
	var el base.ErrorList
	if off == nil {
		return nil
	}
	for i := range files {
		xf := Xfer{
			File:     files[i],
			Transfer: xfer,
		}
		if err := off.Upload(xf); err != nil {
			el.Add(err)
		}
	}
	if el.Empty() {
		return nil
	}
	return el
}
