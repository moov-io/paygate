// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	achClientErrors = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "ach_client_errors",
		Help: "Counter of errors with remote ACH server",
	}, []string{"instance", "operation"})
)

func (a *ACH) trackError(operation string) {
	u, _ := url.Parse(a.endpoint)
	if u == nil {
		achClientErrors.With("instance", "N/A", "operation", operation).Add(1)
	}
	host, port, _ := net.SplitHostPort(u.Host)
	if port == "" {
		port = strings.TrimPrefix(u.Port(), ":")
	}
	achClientErrors.With("instance", fmt.Sprintf("%s:%s", host, port), "operation", operation).Add(1)
}
