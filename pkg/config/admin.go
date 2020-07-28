// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

type Admin struct {
	BindAddress           string
	DisableConfigEndpoint bool
	ExternalURL           string
}
