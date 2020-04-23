// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"fmt"
	"math/rand"
	"time"
	"unicode/utf8"
)

var (
	traceNumberSource = rand.NewSource(time.Now().Unix())
)

// TraceNumber returns a trace number from a given routing number
// and uses a hidden random generator. These values are not expected
// to be cryptographically secure.
func TraceNumber(routingNumber string) string {
	v := fmt.Sprintf("%s%d", ABA8(routingNumber), traceNumberSource.Int63())
	if utf8.RuneCountInString(v) > 15 {
		return v[:15]
	}
	return v
}

// ABA8 returns the first 8 digits of an ABA routing number.
// If the input is invalid then an empty string is returned.
func ABA8(rtn string) string {
	if n := utf8.RuneCountInString(rtn); n == 10 {
		return rtn[1:9] // ACH server will prefix with space, 0, or 1
	}
	if n := utf8.RuneCountInString(rtn); n != 8 && n != 9 {
		return ""
	}
	return rtn[:8]
}

// ABACheckDigit returns the last digit of an ABA routing number.
// If the input is invalid then an empty string is returned.
func ABACheckDigit(rtn string) string {
	if n := utf8.RuneCountInString(rtn); n == 10 {
		return rtn[9:] // ACH server will prefix with space, 0, or 1
	}
	if n := utf8.RuneCountInString(rtn); n != 8 && n != 9 {
		return ""
	}
	return rtn[8:9]
}
