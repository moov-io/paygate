// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package auth

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/moov-io/identity/pkg/logging"
	"github.com/moov-io/identity/pkg/stime"
	"github.com/moov-io/tumbler/pkg/middleware"

	"github.com/go-kit/kit/log"
)

type tumblerExtractor struct {
	mw middleware.TumblerMiddleware
}

func newTumblerExtractor(logger log.Logger, cfg *middleware.TumblerConfig) (*tumblerExtractor, error) {
	if cfg == nil {
		return nil, errors.New("tumbler: nil config")
	}

	log := logging.NewLogger(logger)
	time := stime.NewSystemTimeService()

	mw, err := middleware.NewTumblerMiddlewareFromConfig(log, time, *cfg)
	if err != nil {
		return nil, fmt.Errorf("tumbler: %v", err)
	}

	return &tumblerExtractor{mw: mw}, nil
}

func (ex *tumblerExtractor) TenantID(req *http.Request) string {
	return ""
}
