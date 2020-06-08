// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transform

import (
	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/config"

	"github.com/go-kit/kit/log"
)

type Result struct {
	File      *ach.File
	Encrypted []byte
}

type PreUpload interface {
	Transform(res *Result) (*Result, error)
}

// ForUpload iterates each Transformer over an ACH file and mutates it along the way
func ForUpload(file *ach.File, funcs []PreUpload) (*Result, error) {
	res := &Result{File: file}

	var err error
	for i := range funcs {
		res, err = funcs[i].Transform(res)
		if err != nil {
			return res, err
		}
	}

	return res, nil
}

// Multi is a constructor from our config package for PreUpload transformers
func Multi(logger log.Logger, cfg *config.PreUpload) ([]PreUpload, error) {
	if cfg == nil {
		return nil, nil
	}
	var processors []PreUpload
	if cfg.GPG != nil {
		pc, err := NewGPGEncryptor(logger, cfg.GPG)
		if err != nil {
			return nil, err
		}
		processors = append(processors, pc)
	}
	return processors, nil
}
