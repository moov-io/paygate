// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

// import (
// 	"github.com/go-kit/kit/metrics/prometheus"
// 	stdprometheus "github.com/prometheus/client_golang/prometheus"
// )

// var (
// 	returnFilesProcessed = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
// 		Name: "return_ach_files_processed",
// 		Help: "Counter of return files processed",
// 	}, []string{"origin", "destination", "code"})
// )

// returnFilesProcessed.With("origin", file.Header.ImmediateOrigin, "destination", file.Header.ImmediateDestination,  "code", "").Add(1)

// read returned ACH files
