// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package output

import (
	"bytes"
	"fmt"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/transfers/pipeline/transform"
)

type NACHA struct{}

func (*NACHA) Format(buf *bytes.Buffer, res *transform.Result) error {
	if err := ach.NewWriter(buf).Write(res.File); err != nil {
		return fmt.Errorf("unable to buffer ACH file: %v", err)
	}
	return nil
}
