// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package httptest

import (
	"os"
	"testing"
)

func TestGrabConnectionCertificates(t *testing.T) {
	if testing.Short() {
		return
	}

	path, err := GrabConnectionCertificates(t, "google.com:443")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatalf("%s is an empty file", info.Name())
	}
}
