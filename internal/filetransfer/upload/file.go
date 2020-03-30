// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package upload

import (
	"io"
)

type File struct {
	Filename string
	Contents io.ReadCloser
}

func (f File) Close() error {
	if f.Contents != nil {
		return f.Contents.Close()
	}
	return nil
}
