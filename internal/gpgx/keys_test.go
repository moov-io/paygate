// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package gpgx

import (
	"path/filepath"
	"testing"
)

var (
	password = []byte("password")
)

func TestGPG(t *testing.T) {
	// Encrypt
	pubKey, err := ReadArmoredKeyFile(filepath.Join("testdata", "moov.pub"))
	if err != nil {
		t.Fatal(err)
	}
	msg, err := Encrypt([]byte("hello, world"), pubKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg) == 0 {
		t.Error("empty encrypted message")
	}

	// Decrypt
	privKey, err := ReadPrivateKeyFile(filepath.Join("testdata", "moov.key"), password)
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	out, err := Decrypt(msg, privKey)
	if err != nil {
		t.Fatal(err)
	}

	if v := string(out); v != "hello, world" {
		t.Errorf("got %q", v)
	}
}
