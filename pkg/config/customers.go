// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

type Customers struct {
	Endpoint string   `yaml:"endpoint"`
	Accounts Accounts `yaml:"accounts"`
}

type Accounts struct {
	Decryptor Decryptor `yaml:"decryptor"`
}

type Decryptor struct {
	Symmetric *Symmetric `yaml:"symmetric"`
}

type Symmetric struct {
	KeyURI string `yaml:"keyURI"`
}
