// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func AccountNumber(num string) (string, error) {
	ss := sha256.New()
	n, err := ss.Write([]byte(num))
	if n == 0 || err != nil {
		return "", fmt.Errorf("sha256: n=%d: %v", n, err)
	}
	return hex.EncodeToString(ss.Sum(nil)), nil
}
