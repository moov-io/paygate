// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package sshx

import (
	"encoding/base64"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestSSHX_ReadPubKey(t *testing.T) {
	check := func(t *testing.T, data []byte) {
		key, err := ReadPubKey(data)
		if key == nil || err != nil {
			t.Fatalf("PublicKey=%v error=%v", key, err)
		}

		// base64 Encoded
		data = []byte(base64.StdEncoding.EncodeToString(data))
		key, err = ReadPubKey(data)
		if key == nil || err != nil {
			t.Fatalf("PublicKey=%v error=%v", key, err)
		}
	}

	// Keys generated with 'ssh-keygen -t rsa -b 2048 -f test' (or 4096)
	data, err := ioutil.ReadFile(filepath.Join("testdata", "rsa-2048.pub"))
	if err != nil {
		t.Fatal(err)
	}
	check(t, data)

	data, err = ioutil.ReadFile(filepath.Join("testdata", "rsa-4096.pub"))
	if err != nil {
		t.Fatal(err)
	}
	check(t, data)
}
