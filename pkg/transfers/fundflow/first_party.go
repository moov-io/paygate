// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package fundflow

import (
	"path/filepath"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/client"
)

type firstParty struct{}

func (fp *firstParty) Originate(xfer *client.Transfer, source Source, destination Destination) ([]*ach.File, error) {
	file, err := ach.ReadFile(filepath.Join("testdata", "ppd-debit.ach"))
	return []*ach.File{file}, err
}

func (fp *firstParty) HandleReturn(returned *ach.File, xfer *client.Transfer) ([]*ach.File, error) {
	return nil, nil
}
