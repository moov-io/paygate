// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"io"
)

type SFTPConfig struct {
	Username string
	Password string
	// Key string // in openach code
	IsProduction bool
}

type FileTransferConfig struct {
	InboundPath, InboundFileGlob   string
	OutboundPath, OutboundFileGlob string
	ReturnPath, ReturnFileGlob     string
}

type FileTransferAgent struct {
	config     *FileTransferConfig
	sftpConfig *SFTPConfig
}

func (agent *FileTransferAgent) login() error {
	return nil
}

// TODO(adam): needs filepath? io.ReadCloser ?
func (agent *FileTransferAgent) uploadFile() error {
	return nil
}

type InboundFile struct {
	filename string
	contents io.ReadCloser
}

func (agent *FileTransferAgent) getInboundFiles() ([]InboundFile, error) {
	return nil, nil
}

type ReturnFile struct {
	filename string
	contents io.ReadCloser
}

func (agent *FileTransferAgent) getReturnFiles() ([]ReturnFile, error) {
	return nil, nil
}
