// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/base64"

	ss "github.com/moov-io/paygate/internal/secrets"

	"gocloud.dev/secrets"
)

var (
	testSecretKey    = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("1"), 32))
	testSecretKeeper = func(base64Key string) ss.SecretFunc {
		return func(path string) (*secrets.Keeper, error) {
			return ss.OpenLocal(testSecretKey)
		}
	}
)
