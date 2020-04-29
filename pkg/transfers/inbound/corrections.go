// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

// import (
// 	"github.com/go-kit/kit/metrics/prometheus"
// 	stdprometheus "github.com/prometheus/client_golang/prometheus"
// )

// var (
// 	correctionFilesProcessed = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
// 		Name: "correction_ach_files_processed",
// 		Help: "Counter of correction (COR/NOC) files processed",
// 	}, []string{"origin", "destination", "code"})
// )

// read incoming ACH files (COR/NOC, transfers)
