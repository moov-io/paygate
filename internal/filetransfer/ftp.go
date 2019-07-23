// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

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

type FTPConfig struct {
	RoutingNumber string

	Hostname string
	Username string
	Password string
}

// FTPTransferAgent is an FTP implementation of a Agent
type FTPTransferAgent struct {
	conn *ftp.ServerConn

	// TODO(adam): What sort of metrics should we collect? Just each operation into a histogram?
	// If so we could wrap those in an Agent shim with Prometheus

	cfg        *Config
	ftpConfigs []*FTPConfig

	mu sync.Mutex // protects all read/write methods
}

func (a *FTPTransferAgent) findConfig() *FTPConfig {
	for i := range a.ftpConfigs {
		if a.ftpConfigs[i].RoutingNumber == a.cfg.RoutingNumber {
			return a.ftpConfigs[i]
		}
	}
	return nil
}

func newFTPTransferAgent(cfg *Config, ftpConfigs []*FTPConfig) (*FTPTransferAgent, error) {
	agent := &FTPTransferAgent{cfg: cfg, ftpConfigs: ftpConfigs}
	ftpConf := agent.findConfig()
	if ftpConf == nil {
		return nil, fmt.Errorf("ftp: unable to find config for %s", cfg.RoutingNumber)
	}
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
	agent.conn = conn
	return agent, nil
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

func (agent *FTPTransferAgent) Close() error {
	return agent.conn.Quit()
}

func (agent *FTPTransferAgent) InboundPath() string {
	return agent.cfg.InboundPath
}

func (agent *FTPTransferAgent) OutboundPath() string {
	return agent.cfg.OutboundPath
}

func (agent *FTPTransferAgent) ReturnPath() string {
	return agent.cfg.ReturnPath
}

func (agent *FTPTransferAgent) Delete(path string) error {
	if path == "" || strings.HasSuffix(path, "/") {
		return fmt.Errorf("FTPTransferAgent: invalid path %v", path)
	}
	return agent.conn.Delete(path)
}

// uploadFile saves the content of File at the given filename in the OutboundPath directory
//
// The File's contents will always be closed
func (agent *FTPTransferAgent) UploadFile(f File) error {
	defer f.Close()

	agent.mu.Lock()
	defer agent.mu.Unlock()

	ftpConf := agent.findConfig()
	if ftpConf == nil {
		return fmt.Errorf("ftp.uploadFile: unable to find config for %s", agent.cfg.RoutingNumber)
	}

	// move into inbound directory and set a trigger to undo
	if err := agent.conn.ChangeDir(agent.cfg.OutboundPath); err != nil {
		return err
	}
	defer agent.conn.ChangeDirToParent()

	// Write file contents into path
	return agent.conn.Stor(f.Filename, f.Contents)
}

func (agent *FTPTransferAgent) GetInboundFiles() ([]File, error) {
	return agent.readFiles(agent.cfg.InboundPath)
}

func (agent *FTPTransferAgent) GetReturnFiles() ([]File, error) {
	return agent.readFiles(agent.cfg.ReturnPath)
}

func (agent *FTPTransferAgent) readFiles(path string) ([]File, error) {
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
			Filename: items[i],
			Contents: r,
		})
	}
	return files, nil
}

func (*FTPTransferAgent) readResponse(resp *ftp.Response) (io.ReadCloser, error) {
	defer resp.Close()

	var buf bytes.Buffer
	n, err := io.Copy(&buf, resp)
	if n == 0 || err != nil {
		return ioutil.NopCloser(&buf), fmt.Errorf("n=%d error=%v", n, err)
	}
	return ioutil.NopCloser(&buf), nil
}
