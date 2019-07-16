// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPConfig struct {
	RoutingNumber string

	Hostname string
	Username string

	Password         string
	ClientPrivateKey string
}

type SFTPTransferAgent struct {
	conn   *ssh.Client
	client *sftp.Client

	cfg         *Config
	sftpConfigs []*SFTPConfig

	mu sync.Mutex // protects all read/write methods
}

func (a *SFTPTransferAgent) findConfig() *SFTPConfig {
	for i := range a.sftpConfigs {
		if a.sftpConfigs[i].RoutingNumber == a.cfg.RoutingNumber {
			return a.sftpConfigs[i]
		}
	}
	return nil
}

func newSFTPTransferAgent(cfg *Config, sftpConfigs []*SFTPConfig) (*SFTPTransferAgent, error) {
	agent := &SFTPTransferAgent{cfg: cfg}
	sftpConf := agent.findConfig()
	if sftpConf == nil {
		return nil, fmt.Errorf("sftp: unable to find config for %s", cfg.RoutingNumber)
	}

	conn, err := sshConnect(sftpConf)
	if err != nil {
		return nil, fmt.Errorf("filetransfer: %v", err)
	}
	agent.conn = conn

	// Setup our SFTP client
	var opts = []sftp.ClientOption{
		// TODO(adam): Thoughts on these defaults?
		// // Q(adam): Would we ever have multiple requests to the same file?
		// // See: https://godoc.org/github.com/pkg/sftp#MaxConcurrentRequestsPerFile
		// sftp.MaxConcurrentRequestsPerFile(64),
		// // The docs suggest lowering this on "failed to send packet header: EOF" errors,
		// // so we're going to lower it by default (which is 32768).
		// sftp.MaxPacket(29999),
	}
	client, err := sftp.NewClient(agent.conn, opts...)
	if err != nil {
		return nil, fmt.Errorf("filetransfer: sftp connect: %v", err)
	}
	agent.client = client

	return agent, nil
}

func sshConnect(sftpConf *SFTPConfig) (*ssh.Client, error) {
	conf := &ssh.ClientConfig{
		User:    sftpConf.Username,
		Timeout: 30 * time.Second,
		// TODO(adam): How to read this per-host?
		// var hostKey ssh.PublicKey
		// HostKeyCallback: ssh.FixedHostKey(hostKey)
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO(adam): fix
	}
	switch {
	case sftpConf.Password != "":
		conf.Auth = append(conf.Auth, ssh.Password(sftpConf.Password))
	case sftpConf.ClientPrivateKey != "":
		// TODO(adam): attempt base64 decode also
		signer, err := ssh.ParsePrivateKey([]byte(sftpConf.ClientPrivateKey))
		if err != nil {
			return nil, fmt.Errorf("sshConnect: failed to read client private key: %v", err)
		}
		conf.Auth = append(conf.Auth, ssh.PublicKeys(signer))
	default:
		return nil, fmt.Errorf("sshConnect: no auth method provided for routingNumber=%s", sftpConf.RoutingNumber)
	}

	// Connect to the remote server
	client, err := ssh.Dial("tcp", sftpConf.Hostname, conf)
	if err != nil {
		return nil, fmt.Errorf("sshConnect: error with routingNumber=%s: %v", sftpConf.RoutingNumber, err)
	}
	return client, nil
}

func (a *SFTPTransferAgent) Close() error {
	if a == nil {
		return nil
	}
	e1 := a.client.Close()
	e2 := a.conn.Close()
	if e1 != nil || e2 != nil {
		return fmt.Errorf("sftp: agent close e1=%v e2=%v", e1, e2)
	}
	return nil
}

func (agent *SFTPTransferAgent) InboundPath() string {
	return agent.cfg.InboundPath
}

func (agent *SFTPTransferAgent) OutboundPath() string {
	return agent.cfg.OutboundPath
}

func (agent *SFTPTransferAgent) ReturnPath() string {
	return agent.cfg.ReturnPath
}

func (agent *SFTPTransferAgent) Delete(path string) error {
	info, err := agent.client.Stat(path)
	if err != nil {
		return fmt.Errorf("sftp: delete stat: %v", err)
	}
	if info != nil {
		if err := agent.client.Remove(path); err != nil {
			return fmt.Errorf("sftp: delete: %v", err)
		}
	}
	return nil // not found
}

// uploadFile saves the content of File at the given filename in the OutboundPath directory
//
// The File's contents will always be closed
func (agent *SFTPTransferAgent) UploadFile(f File) error {
	defer f.Close()

	agent.mu.Lock()
	defer agent.mu.Unlock()

	fd, err := agent.client.Create(f.Filename)
	if err != nil {
		return fmt.Errorf("sftp: problem creating %s: %v", f.Filename, err)
	}
	n, err := io.Copy(fd, f.Contents)
	if n == 0 || err != nil {
		return fmt.Errorf("sftp: problem copying (n=%d) %s: %v", n, f.Filename, err)
	}
	if err := fd.Close(); err != nil {
		return fmt.Errorf("sftp: problem closing %s: %v", f.Filename, err)
	}
	if err := fd.Chmod(0600); err != nil {
		return fmt.Errorf("sftp: problem chmod %s: %v", f.Filename, err)
	}
	return nil
}

func (agent *SFTPTransferAgent) GetInboundFiles() ([]File, error) {
	return agent.readFiles(agent.cfg.InboundPath)
}

func (agent *SFTPTransferAgent) GetReturnFiles() ([]File, error) {
	return agent.readFiles(agent.cfg.ReturnPath)
}

func (agent *SFTPTransferAgent) readFiles(dir string) ([]File, error) {
	infos, err := agent.client.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("sftp: readdir %s: %v", dir, err)
	}

	var files []File
	for i := range infos {
		fd, err := agent.client.Open(infos[i].Name())
		if err != nil {
			return nil, fmt.Errorf("sftp: open %s: %v", infos[i].Name(), err)
		}
		var buf bytes.Buffer
		if n, err := io.Copy(&buf, fd); n == 0 || err != nil {
			fd.Close()
			return nil, fmt.Errorf("sftp: read (n=%d) %s: %v", n, infos[i].Name(), err)
		}
		fd.Close()
		files = append(files, File{
			Filename: infos[i].Name(),
			Contents: ioutil.NopCloser(&buf),
		})
	}
	return files, nil
}
