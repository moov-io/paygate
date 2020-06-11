// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package gpgx

import (
	"path/filepath"
	"testing"

	"github.com/moov-io/paygate/internal/sshx"
)

func TestFromSSHPublicKey2048(t *testing.T) {
	pk, err := sshx.ReadPubKeyFile(filepath.Join("..", "sshx", "testdata", "rsa-2048.pub"))
	if err != nil {
		t.Fatal(err)
	}

	key := FromSSHPublicKey(pk)
	if key == nil {
		t.Fatal("nil openpgp.EntityList")
	}

	msg, err := Encrypt([]byte("hello, world"), key)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg) == 0 {
		t.Error("empty encrypted message")
	}
}

func TestFromSSHPublicKey4096(t *testing.T) {
	pk, err := sshx.ReadPubKeyFile(filepath.Join("..", "sshx", "testdata", "rsa-4096.pub"))
	if err != nil {
		t.Fatal(err)
	}

	key := FromSSHPublicKey(pk)
	if key == nil {
		t.Fatal("nil openpgp.EntityList")
	}

	msg, err := Encrypt([]byte("hello, world"), key)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg) == 0 {
		t.Error("empty encrypted message")
	}
}
