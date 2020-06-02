// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package upload

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"

	"github.com/moov-io/paygate/pkg/config"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/prometheus"
	"github.com/jlaffaye/ftp"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	ftpAgentUp = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Name: "ftp_agent_up",
		Help: "Status of FTP agent connection ",
	}, []string{"hostname"})
)

// FTPTransferAgent is an FTP implementation of a Agent
type FTPTransferAgent struct {
	conn   *ftp.ServerConn
	cfg    config.ODFI
	logger log.Logger
	mu     sync.Mutex // protects all read/write methods
}

// TODO(adam): What sort of metrics should we collect? Just each operation into a histogram?
// If so we could wrap those in an Agent shim with Prometheus

func newFTPTransferAgent(logger log.Logger, cfg config.ODFI) (*FTPTransferAgent, error) {
	if cfg.FTP == nil {
		return nil, errors.New("nil FTP config")
	}
	agent := &FTPTransferAgent{
		cfg:    cfg,
		logger: logger,
	}

	if err := rejectOutboundIPRange(cfg.SplitAllowedIPs(), cfg.FTP.Hostname); err != nil {
		return nil, fmt.Errorf("ftp: %s is not whitelisted: %v", cfg.FTP.Hostname, err)
	}

	_, err := agent.connection() // initial connection

	return agent, err
}

// connection returns an ftp.ServerConn which is connected to the remote server.
// This function will attempt to establish a new connection if none exists already.
//
// connection must be called within a mutex lock as the underlying FTP client is not
// goroutine-safe.
func (agent *FTPTransferAgent) connection() (*ftp.ServerConn, error) {
	if agent == nil || agent.cfg.FTP == nil {
		return nil, errors.New("nil agent / config")
	}

	if agent.conn != nil {
		// Verify the connection works and f not drop through and reconnect
		if err := agent.conn.NoOp(); err == nil {
			return agent.conn, nil
		} else {
			// Our connection is having issues, so retry connecting
			agent.conn.Quit()
		}
	}

	// Setup our FTP connection
	opts := []ftp.DialOption{
		ftp.DialWithTimeout(agent.cfg.FTP.Timeout()),
		ftp.DialWithDisabledEPSV(agent.cfg.FTP.DisableEPSV()),
	}
	tlsOpt, err := tlsDialOption(agent.cfg.FTP.CAFile())
	if err != nil {
		return nil, err
	}
	if tlsOpt != nil {
		opts = append(opts, *tlsOpt)
	}

	// Make the first connection
	conn, err := ftp.Dial(agent.cfg.FTP.Hostname, opts...)
	if err != nil {
		return nil, err
	}
	if err := conn.Login(agent.cfg.FTP.Username, agent.cfg.FTP.Password); err != nil {
		return nil, err
	}
	agent.conn = conn

	return agent.conn, nil
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

func (agent *FTPTransferAgent) Ping() error {
	if agent == nil {
		return errors.New("nil FTPTransferAgent")
	}

	agent.mu.Lock()
	defer agent.mu.Unlock()

	conn, err := agent.connection()
	agent.record(err)
	if err != nil {
		return err
	}

	err = conn.NoOp()
	agent.record(err)
	return err
}

func (agent *FTPTransferAgent) record(err error) {
	if agent == nil || agent.cfg.FTP == nil {
		return
	}
	if err != nil {
		ftpAgentUp.With("hostname", agent.cfg.FTP.Hostname).Set(0)
	} else {
		ftpAgentUp.With("hostname", agent.cfg.FTP.Hostname).Set(1)
	}
}

func (agent *FTPTransferAgent) Close() error {
	if agent == nil || agent.conn == nil {
		return nil
	}

	agent.mu.Lock()
	defer agent.mu.Unlock()

	conn, err := agent.connection()
	if err != nil {
		return err
	}
	return conn.Quit()
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
	agent.mu.Lock()
	defer agent.mu.Unlock()

	if path == "" || strings.HasSuffix(path, "/") {
		return fmt.Errorf("FTPTransferAgent: invalid path %v", path)
	}

	conn, err := agent.connection()
	if err != nil {
		return err
	}
	return conn.Delete(path)
}

// uploadFile saves the content of File at the given filename in the OutboundPath directory
//
// The File's contents will always be closed
func (agent *FTPTransferAgent) UploadFile(f File) error {
	defer f.Close()

	agent.mu.Lock()
	defer agent.mu.Unlock()

	conn, err := agent.connection()
	if err != nil {
		return err
	}

	// move into inbound directory and set a trigger to undo and set a defer to move back
	wd, err := conn.CurrentDir()
	if err != nil {
		return err
	}
	if err := conn.ChangeDir(agent.cfg.OutboundPath); err != nil {
		return err
	}
	defer func(path string) {
		// Return to our previous directory when initially called
		if err := conn.ChangeDir(path); err != nil {
			agent.logger.Log("ftp", fmt.Sprintf("FTP: problem uploading file: %v", err))
		}
	}(wd)

	// Write file contents into path
	// Take the base of f.Filename and our (out of band) OutboundPath to avoid accepting a write like '../../../../etc/passwd'.
	return conn.Stor(filepath.Base(f.Filename), f.Contents)
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

	conn, err := agent.connection()
	if err != nil {
		return nil, err
	}

	// move into inbound directory and set a trigger to undo
	wd, err := conn.CurrentDir()
	if err != nil {
		return nil, err
	}
	defer func(path string) {
		// Return to our previous directory when initially called
		if err := conn.ChangeDir(wd); err != nil {
			agent.logger.Log("ftp", fmt.Sprintf("FTP: problem with readFiles: %v", err))
		}
	}(wd)
	if err := conn.ChangeDir(path); err != nil {
		return nil, err
	}

	items, err := conn.NameList("")
	if err != nil {
		return nil, err
	}
	var files []File
	for i := range items {
		resp, err := conn.Retr(items[i])
		if err != nil {
			return nil, fmt.Errorf("problem retrieving %s: %v", items[i], err)
		}

		r, err := agent.readResponse(resp)
		if err != nil {
			return nil, fmt.Errorf("problem reading %s: %v", items[i], err)
		}
		if r != nil {
			files = append(files, File{
				Filename: items[i],
				Contents: r,
			})
		}
	}
	return files, nil
}

func (*FTPTransferAgent) readResponse(resp *ftp.Response) (io.ReadCloser, error) {
	defer resp.Close()

	var buf bytes.Buffer
	n, err := io.Copy(&buf, resp)
	// If there was nothing downloaded and no error then assume it's a directory.
	//
	// The FTP client doesn't have a STAT command, so we can't quite ensure this
	// was a directory.
	//
	// See https://github.com/moov-io/paygate/issues/494
	if n == 0 && err == nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("n=%d error=%v", n, err)
	}
	return ioutil.NopCloser(&buf), nil
}
