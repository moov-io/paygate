// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package output

import (
	"bytes"

	"github.com/moov-io/paygate/pkg/transfers/pipeline/transform"
)

type Encrypted struct{}

func (*Encrypted) Format(buf *bytes.Buffer, res *transform.Result) error {
	buf.Write(res.Encrypted)
	return nil
}
