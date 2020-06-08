// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestBase64(t *testing.T) {
	enc := &Base64{}

	var buf bytes.Buffer
	res := testResult(t)

	if err := enc.Format(&buf, res); err != nil {
		t.Fatal(err)
	}

	if buf.Len() == 0 {
		t.Error("encoded zero bytes")
	}

	if !strings.HasPrefix(buf.String(), `MTAxIDA3NjQwMTI1MSAwNzY0MDEyNTE`) {
		t.Errorf("unexpected output: %v", buf.String())
	}
}

func TestBase64Encrypted(t *testing.T) {
	enc := &Base64{}

	var buf bytes.Buffer
	res := testResult(t)
	res.Encrypted = []byte("hello, world")

	if err := enc.Format(&buf, res); err != nil {
		t.Fatal(err)
	}

	if buf.Len() == 0 {
		t.Error("encoded zero bytes")
	}

	if !strings.HasPrefix(buf.String(), `aGVsbG8sIHdvcmxk`) {
		t.Errorf("unexpected output: %v", buf.String())
	}
}
