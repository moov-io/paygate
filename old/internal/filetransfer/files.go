// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"github.com/moov-io/ach"
)

func collectTraceNumbers(f *ach.File) []string {
	var out []string
	for i := range f.Batches {
		entries := f.Batches[i].GetEntries()
		for k := range entries {
			out = append(out, entries[k].TraceNumber)
		}
	}
	for i := range f.IATBatches {
		for k := range f.IATBatches[i].Entries {
			out = append(out, f.IATBatches[i].Entries[k].TraceNumber)
		}
	}
	return out
}
