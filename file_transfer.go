// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
)

type SFTPConfig struct {
	Hostname           string
	Username, Password string
}

type FileTransferConfig struct {
	InboundPath  string
	OutboundPath string
	ReturnPath   string
}

type FileTransferAgent struct {
	config *FileTransferConfig
	conn   *ftp.ServerConn

	mu sync.Mutex // protects all read/write methods
}

func (agent *FileTransferAgent) close() error {
	return agent.conn.Quit()
}

func NewFileTransfer(sftpConf *SFTPConfig, conf *FileTransferConfig) (*FileTransferAgent, error) {
	conn, err := ftp.DialTimeout(sftpConf.Hostname, 30*time.Second)
	if err != nil {
		return nil, err
	}
	if err := conn.Login(sftpConf.Username, sftpConf.Password); err != nil {
		return nil, err
	}
	return &FileTransferAgent{
		config: conf,
		conn:   conn,
	}, nil
}

// TODO(adam): needs filepath? io.ReadCloser ?
func (agent *FileTransferAgent) uploadFile() error {
	return nil
}

type File struct {
	filename string
	contents io.ReadCloser
}

func (agent *FileTransferAgent) getInboundFiles() ([]File, error) {
	return agent.readFiles(agent.config.InboundPath)
}

func (agent *FileTransferAgent) getReturnFiles() ([]File, error) {
	return agent.readFiles(agent.config.ReturnPath)
}

func (agent *FileTransferAgent) readFiles(path string) ([]File, error) {
	agent.mu.Lock()
	defer agent.mu.Unlock()

	// move into inbound directory and set a trigger to undo
	if err := agent.conn.ChangeDir(path); err != nil {
		return nil, err
	}
	defer agent.conn.ChangeDirToParent()

	// Read files in current directory
	items, err := agent.conn.NameList("")
	if err != nil {
		return nil, err
	}
	var files []File
	for i := range items {
		resp, err := agent.conn.Retr(items[i])
		if err != nil {
			return nil, fmt.Errorf("problem retrieving %s: %v", items[i], err)
		}
		r, err := agent.readResponse(resp)
		if err != nil {
			return nil, fmt.Errorf("problem reading %s: %v", items[i], err)
		}
		files = append(files, File{
			filename: items[i],
			contents: r,
		})
	}
	return files, nil
}

func (agent *FileTransferAgent) readResponse(resp *ftp.Response) (io.ReadCloser, error) {
	defer resp.Close()

	var buf bytes.Buffer
	n, err := io.Copy(&buf, resp)
	if n == 0 || err != nil {
		return ioutil.NopCloser(&buf), fmt.Errorf("n=%d error=%v", n, err)
	}
	return ioutil.NopCloser(&buf), nil
}
