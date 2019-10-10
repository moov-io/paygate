// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"testing"
)

func TestFilenameTemplate(t *testing.T) {
	t.Logf("orig: %s", achFilename("987654320", 1))
	t.Logf("")

	cases := []struct {
		name, raw string
	}{
		{name: "tmpl", raw: defaultFilenameTemplate},
		{name: "encr", raw: encrypted},
		{name: "lind", raw: lindenExample},
	}
	for i := range cases {
		res, err := build(cases[i].name, cases[i].raw)
		if err != nil {
			t.Fatalf("%s: %v", cases[i].name, err)
		}
		t.Logf("%s: %s", cases[i].name, res)
	}
}
