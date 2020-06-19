// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestProcessor__process(t *testing.T) {
	dir := testDir(t)
	if err := ioutil.WriteFile(filepath.Join(dir, "invalid.ach"), []byte("invalid-ach-file"), 0644); err != nil {
		t.Fatal(err)
	}

	processors := SetupProcessors(&MockProcessor{})

	// By reading a file without ACH FileHeaders we still want to try and process
	// Batches inside of it if any are found, so reading this kind of file shouldn't
	// return an error from reading the file.
	if err := process(dir, processors); err != nil {
		t.Error(err)
	}
}
