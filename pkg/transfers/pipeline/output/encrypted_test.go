// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package output

import (
	"bytes"
	"testing"
)

func TestEncrypted(t *testing.T) {
	enc := &Encrypted{}

	var buf bytes.Buffer
	res := testResult(t)
	res.Encrypted = []byte("hello, world")

	if err := enc.Format(&buf, res); err != nil {
		t.Fatal(err)
	}

	if s := buf.String(); s != "hello, world" {
		t.Errorf("unexpected output: %q", s)
	}
}
