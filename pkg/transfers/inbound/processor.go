// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
)

type FileProcessor interface {
	Type() string

	// Handle processes an ACH file with whatever logic is implemented
	Handle(file *ach.File) error
}

type Processors []FileProcessor

func SetupProcessors(pcs ...FileProcessor) Processors {
	return Processors(pcs)
}

func (pcs Processors) HandleAll(file *ach.File) error {
	var el base.ErrorList
	for i := range pcs {
		if err := pcs[i].Handle(file); err != nil {
			el.Add(fmt.Errorf("%s: %v", pcs[i].Type(), err))
		}
	}
	if el.Empty() {
		return nil
	}
	return el
}

func ProcessFiles(dl *downloadedFiles, fileProcessors Processors) error {
	var el base.ErrorList
	fds, err := ioutil.ReadDir(dl.dir)
	if err != nil {
		return fmt.Errorf("reading %s: %v", dl.dir, err)
	}
	for i := range fds {
		file, err := ach.ReadFile(filepath.Join(dl.dir, fds[i].Name()))
		if err != nil {
			// Some return files don't contain FileHeader info, but can be processed as there
			// are batches with entries. Let's continue to process those, but skip other errors.
			if !base.Has(err, ach.ErrFileHeader) {
				el.Add(fmt.Errorf("problem opening %s: %v", fds[i].Name(), err))
				continue
			}
		}
		if err := fileProcessors.HandleAll(file); err != nil {
			el.Add(fmt.Errorf("processing %s error: %v", fds[i].Name(), err))
			continue
		}
	}

	if el.Empty() {
		return nil
	}
	return el

}
