// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/client"
)

type Xfer struct {
	Transfer *client.Transfer `json:"transfer"`
	File     *ach.File        `json:"file"`
}

type CanceledTransfer struct {
	TransferID string
}
