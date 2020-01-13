// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package id

type Depository string

func (d Depository) String() string {
	return string(d)
}

type User string

func (u User) String() string {
	return string(u)
}
