// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package upload

import (
	"bytes"
	"io/ioutil"
	"sync"
)

type MockAgent struct {
	InboundFiles []File
	ReturnFiles  []File
	UploadedFile *File        // non-nil on file upload
	DeletedFile  string       // filepath of last deleted file
	mu           sync.RWMutex // protects all fields

	Err error
}

func (a *MockAgent) GetInboundFiles() ([]File, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.InboundFiles, nil
}

func (a *MockAgent) GetReturnFiles() ([]File, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.ReturnFiles, nil
}

func (a *MockAgent) UploadFile(f File) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// read f.contents before callers close the underlying os.Open file descriptor
	bs, _ := ioutil.ReadAll(f.Contents)
	a.UploadedFile = &f
	a.UploadedFile.Contents = ioutil.NopCloser(bytes.NewReader(bs))
	return nil
}

func (a *MockAgent) Delete(path string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.DeletedFile = path
	return nil
}

func (a *MockAgent) InboundPath() string {
	return "inbound/"
}

func (a *MockAgent) OutboundPath() string {
	return "outbound/"
}

func (a *MockAgent) ReturnPath() string {
	return "return/"
}

func (a *MockAgent) Ping() error {
	return a.Err
}

func (a *MockAgent) Close() error {
	return nil
}
