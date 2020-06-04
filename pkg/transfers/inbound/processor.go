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
			el.Add(fmt.Errorf("%T: %v", pcs[i], err))
		}
	}
	if el.Empty() {
		return nil
	}
	return el
}

func ProcessFiles(dl *downloadedFiles, fileProcessors Processors) error {
	var el base.ErrorList
	dirs, err := ioutil.ReadDir(dl.dir)
	if err != nil {
		return fmt.Errorf("reading %s: %v", dl.dir, err)
	}
	for i := range dirs {
		if err := process(filepath.Join(dl.dir, dirs[i].Name()), fileProcessors); err != nil {
			el.Add(fmt.Errorf("%s: %v", dirs[i], err))
		}
	}
	if el.Empty() {
		return nil
	}
	return el
}

func process(dir string, fileProcessors Processors) error {
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading %s: %v", dir, err)
	}

	var el base.ErrorList
	for i := range infos {
		file, err := ach.ReadFile(filepath.Join(dir, infos[i].Name()))
		if err != nil {
			el.Add(fmt.Errorf("problem opening %s: %v", infos[i].Name(), err))
			continue
		}
		if err := fileProcessors.HandleAll(file); err != nil {
			el.Add(fmt.Errorf("processing %s error: %v", infos[i].Name(), err))
			continue
		}
	}

	if el.Empty() {
		return nil
	}
	return el
}
