// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package output

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/transfers/pipeline/transform"
)

func testResult(t *testing.T) *transform.Result {
	t.Helper()

	path := filepath.Join("..", "..", "..", "..", "testdata", "ppd-debit.ach")

	file, err := ach.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return &transform.Result{
		File: file,
	}
}

func TestNACHA(t *testing.T) {
	enc := &NACHA{}

	var buf bytes.Buffer
	res := testResult(t)

	if err := enc.Format(&buf, res); err != nil {
		t.Fatal(err)
	}

	if s := buf.String(); !strings.HasPrefix(s, `101 076401251 076401251080729`) {
		t.Errorf("unexpected output:\n%v", s)
	}
}
