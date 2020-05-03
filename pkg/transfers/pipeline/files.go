// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
)

// PublishFiles attempts to upload all files to the Pipeline and returns
// all errors as a base.ErrorList.
//
// All files are attempted to be published as downstream processors
// are expected to de-duplicate files.
func PublishFiles(pub XferPublisher, xfer *client.Transfer, files []*ach.File) error {
	if pub == nil {
		return nil
	}

	var el base.ErrorList
	for i := range files {
		xf := Xfer{
			File:     files[i],
			Transfer: xfer,
		}
		if err := pub.Upload(xf); err != nil {
			el.Add(err)
		}
	}
	if el.Empty() {
		return nil
	}
	return el
}
