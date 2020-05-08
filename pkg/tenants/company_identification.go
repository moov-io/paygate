// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package tenants

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
)

var (
	encoder = base32.StdEncoding
)

const (
	maxLength = 10 // CompanyIdentification is a 10-digit BatchHeader field
)

// CompanyIdentification returns a string formatted for the CompanyIdentification field in an ACH
// Batc Header record. This is used to organize Batches/Entries at the ODFI.
func CompanyIdentification(prefix string) string {
	if len(prefix) >= maxLength {
		return prefix[:maxLength]
	}

	length := maxLength - len(prefix)

	bs := make([]byte, length)

	if n, err := rand.Read(bs); n == 0 || err != nil {
		return ""
	}

	return fmt.Sprintf("%s%s", prefix, strings.ToUpper(encoder.EncodeToString(bs))[:length])
}
