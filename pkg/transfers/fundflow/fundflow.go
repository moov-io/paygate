// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package fundflow

// New ... // TODO(adam):
func New() Strategy {
	return &firstParty{}
}
