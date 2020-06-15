// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package audittrail

import (
	"github.com/moov-io/ach"
)

type MockStorage struct {
	Err error
}

func (s *MockStorage) Close() error {
	return s.Err
}

func (s *MockStorage) SaveFile(filename string, file *ach.File) error {
	return s.Err
}
