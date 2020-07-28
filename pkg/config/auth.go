// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"github.com/moov-io/tumbler/pkg/middleware"
)

type Auth struct {
	Tenants Tenants
}

type Tenants struct {
	Headers *Headers
	Tumbler *middleware.TumblerConfig
}

type Headers struct {
	Names []string
}
