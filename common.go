// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

type AccountType int

func (t AccountType) String() string {
	switch t {
	case Checking:
		return "Checking"
	case Savings:
		return "Savings"
	}
	return "unknown"
}

const (
	Checking AccountType = iota
	Savings
)

type Amount *big.Rat

// nextID creates a new ID for our system.
// Do no assume anything about these ID's other than
// they are strings. Case matters!
func nextID() string {
	bs := make([]byte, 20)
	n, err := rand.Read(bs)
	if err != nil || n == 0 {
		logger.Log("generateID", fmt.Sprintf("n=%d, err=%v", n, err))
		return ""
	}
	return strings.ToLower(hex.EncodeToString(bs))
}
