// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
)

type ftpConfig struct {
	RoutingNumber string

	Hostname           string
	Username, Password string
}

type fileTransferConfig struct {
	RoutingNumber string

	InboundPath  string
	OutboundPath string
	ReturnPath   string
}

// fileTransferAgent represents an interface for uploading and retrieving ACH files from a remote service.
type fileTransferAgent interface {
	getInboundFiles() ([]file, error)
	getReturnFiles() ([]file, error)
	uploadFile(f file) error
	delete(path string) error

	inboundPath() string
	outboundPath() string
	returnPath() string

	close() error
}

// ftpFileTransferAgent is an FTP implementation of a fileTransferAgent
type ftpFileTransferAgent struct {
	config *fileTransferConfig
	conn   *ftp.ServerConn

	mu sync.Mutex // protects all read/write methods
}

func (agent *ftpFileTransferAgent) inboundPath() string {
	return agent.config.InboundPath
}

func (agent *ftpFileTransferAgent) outboundPath() string {
	return agent.config.OutboundPath
}

func (agent *ftpFileTransferAgent) returnPath() string {
	return agent.config.ReturnPath
}

func (agent *ftpFileTransferAgent) close() error {
	return agent.conn.Quit()
}

// newFileTransferAgent returns an FTP implementation of a fileTransferAgent
//
// This function reads ACH_FILE_TRANSFERS_ROOT_CAFILE for a file with root certificates to be used
// in all secured connections.
func newFileTransferAgent(ftpConf *ftpConfig, conf *fileTransferConfig) (fileTransferAgent, error) {
	opts := []ftp.DialOption{
		ftp.DialWithTimeout(30 * time.Second),
		ftp.DialWithDisabledEPSV(false),
	}
	tlsOpt, err := tlsDialOption(os.Getenv("ACH_FILE_TRANSFERS_CAFILE"))
	if err != nil {
		return nil, err
	}
	if tlsOpt != nil {
		opts = append(opts, *tlsOpt)
	}
	// Make the first connection
	conn, err := ftp.Dial(ftpConf.Hostname, opts...)
	if err != nil {
		return nil, err
	}

	if err := conn.Login(ftpConf.Username, ftpConf.Password); err != nil {
		return nil, err
	}
	return &ftpFileTransferAgent{
		config: conf,
		conn:   conn,
	}, nil
}

func tlsDialOption(caFilePath string) (*ftp.DialOption, error) {
	if caFilePath == "" {
		return nil, nil
	}
	bs, err := ioutil.ReadFile(caFilePath)
	if err != nil {
		return nil, fmt.Errorf("tlsDialOption: failed to read %s: %v", caFilePath, err)
	}
	pool, err := x509.SystemCertPool()
	if pool == nil || err != nil {
		pool = x509.NewCertPool()
	}
	ok := pool.AppendCertsFromPEM(bs)
	if !ok {
		return nil, fmt.Errorf("tlsDialOption: problem with AppendCertsFromPEM from %s", caFilePath)
	}
	cfg := &tls.Config{
		RootCAs: pool,
	}
	opt := ftp.DialWithTLS(cfg)
	return &opt, nil
}

func (agent *ftpFileTransferAgent) delete(path string) error {
	if path == "" || strings.HasSuffix(path, "/") {
		return fmt.Errorf("ftpFileTransferAgent: invalid path %v", path)
	}
	return agent.conn.Delete(path)
}

// uploadFile saves the content of File at the given filename in the OutboundPath directory
//
// The File's contents will always be closed
func (agent *ftpFileTransferAgent) uploadFile(f file) error {
	agent.mu.Lock()
	defer agent.mu.Unlock()
	defer f.contents.Close() // close File

	// move into inbound directory and set a trigger to undo
	if err := agent.conn.ChangeDir(agent.config.OutboundPath); err != nil {
		return err
	}
	defer agent.conn.ChangeDirToParent()

	// Write file contents into path
	return agent.conn.Stor(f.filename, f.contents)
}

type file struct {
	filename string
	contents io.ReadCloser
}

func (agent *ftpFileTransferAgent) getInboundFiles() ([]file, error) {
	return agent.readFiles(agent.config.InboundPath)
}

func (agent *ftpFileTransferAgent) getReturnFiles() ([]file, error) {
	return agent.readFiles(agent.config.ReturnPath)
}

func (agent *ftpFileTransferAgent) readFiles(path string) ([]file, error) {
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
	var files []file
	for i := range items {
		resp, err := agent.conn.Retr(items[i])
		if err != nil {
			return nil, fmt.Errorf("problem retrieving %s: %v", items[i], err)
		}
		r, err := agent.readResponse(resp)
		if err != nil {
			return nil, fmt.Errorf("problem reading %s: %v", items[i], err)
		}
		files = append(files, file{
			filename: items[i],
			contents: r,
		})
	}
	return files, nil
}

func (agent *ftpFileTransferAgent) readResponse(resp *ftp.Response) (io.ReadCloser, error) {
	defer resp.Close()

	var buf bytes.Buffer
	n, err := io.Copy(&buf, resp)
	if n == 0 || err != nil {
		return ioutil.NopCloser(&buf), fmt.Errorf("n=%d error=%v", n, err)
	}
	return ioutil.NopCloser(&buf), nil
}
