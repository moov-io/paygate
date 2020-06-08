// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package output

import (
	"bytes"
	"encoding/base64"

	"github.com/moov-io/paygate/pkg/transfers/pipeline/transform"
)

type Base64 struct{}

// Format converts any encrypted bytes into standard Base64 encoding. If no encrypted
// bytes are passed then the file is encoded with NACHA formatting and then Base64 encoded.
func (*Base64) Format(buf *bytes.Buffer, res *transform.Result) error {
	if len(res.Encrypted) > 0 {
		buf.WriteString(base64.StdEncoding.EncodeToString(res.Encrypted))
	} else {
		var buf2 bytes.Buffer

		nacha := &NACHA{}
		if err := nacha.Format(&buf2, res); err != nil {
			return err
		}

		buf.WriteString(base64.StdEncoding.EncodeToString(buf2.Bytes()))
	}
	return nil
}
