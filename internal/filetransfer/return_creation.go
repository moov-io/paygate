// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/remoteach"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	returnFilesCreated = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "return_ach_files_created",
		Help: "Counter of return files created",
	}, []string{"destination", "origin"}) // , "code"
)

// TODO(adam): rename to "uploadReturnFile(s)" ??

func (c *Controller) writeReturnFile(cfg *Config, file *ach.File) error {
	returnFilesCreated.With("destination", file.Header.ImmediateDestination, "origin", file.Header.ImmediateOrigin).Add(1)

	filename, err := renderACHFilename(cfg.outboundFilenameTemplate(), filenameData{
		RoutingNumber: file.Header.ImmediateDestination,
		N:             "1",
	})
	if err != nil {
		return fmt.Errorf("problem writing routingNumber=%s return file: %v", cfg.RoutingNumber, err)
	}
	filename = fmt.Sprintf("return-%s", filename)

	// write and upload return file
	scratch := c.scratchDir()
	out := &achFile{
		File:     file,
		filepath: filepath.Join(scratch, filename),
	}
	if err := out.write(); err != nil {
		return fmt.Errorf("problem writing file for return: %v", err)
	}
	if err := c.maybeUploadFile(out); err != nil {
		return fmt.Errorf("problem uploading return file: %v", err)
	}
	return nil
}

func (c *Controller) writeReturnFiles(cfg *Config, files []*ach.File) error {
	for i := range files {
		if err := c.writeReturnFile(cfg, files[i]); err != nil {
			return err
		}
	}
	return nil
}

func returnEntireFile(file *ach.File, code string) ([]*ach.File, error) {
	var acc []*ach.File
	var err error

	for i := range file.Batches {
		entries := file.Batches[i].GetEntries()
		for j := range entries {
			f, err := returnEntry(file.Header, file.Batches[i], entries[j], code)
			if err != nil {
				return nil, fmt.Errorf("problem returning=%s traceNumber=%s", code, entries[j].TraceNumber)
			}
			acc, err = ach.MergeFiles(append(acc, f))
			if err != nil {
				return nil, fmt.Errorf("problem merging return files: %v", err)
			}
		}
	}

	return acc, err
}

func returnEntry(fh ach.FileHeader, b ach.Batcher, entry *ach.EntryDetail, code string) (*ach.File, error) {
	returnCode := ach.LookupReturnCode(code)
	if returnCode == nil {
		return nil, fmt.Errorf("unknown return code: %s", code)
	}

	addenda99 := ach.NewAddenda99()
	addenda99.ReturnCode = returnCode.Code
	addenda99.OriginalTrace = entry.TraceNumber
	addenda99.OriginalDFI = entry.RDFIIdentification
	// set TraceNumber as ODFI (we are returning)
	addenda99.TraceNumber = remoteach.CreateTraceNumber(fh.ImmediateDestination)

	file := ach.NewFile()
	file.Header = fh

	// swap origin / destination
	file.Header.ImmediateOrigin = fh.ImmediateDestination
	file.Header.ImmediateOriginName = fh.ImmediateDestinationName
	file.Header.ImmediateDestination = fh.ImmediateOrigin
	file.Header.ImmediateDestinationName = fh.ImmediateOriginName

	now := time.Now()
	file.Header.FileCreationDate = now.Format("060102") // YYMMDD
	file.Header.FileCreationTime = now.Format("1504")   // HHMM

	batchHeader := b.GetHeader()
	batchHeader.EffectiveEntryDate = now.Format("060102") // YYMMDD

	// Adjust EntryDetail we're going to return
	entry.RDFIIdentification = remoteach.ABA8(file.Header.ImmediateDestination)
	entry.CheckDigit = remoteach.ABACheckDigit(file.Header.ImmediateDestination)

	batch, err := ach.NewBatch(batchHeader)
	if err != nil {
		return nil, err
	}
	batch.AddEntry(entry)
	if err := batch.Create(); err != nil {
		return nil, err
	}

	file.AddBatch(batch)
	if err := file.Create(); err != nil {
		return nil, err
	}

	return file, nil
}
