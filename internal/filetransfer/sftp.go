// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var (
	sftpDialTimeout = func() time.Duration {
		if v := os.Getenv("SFTP_DIAL_TIMEOUT"); v != "" {
			if dur, _ := time.ParseDuration(v); dur > 0 {
				return dur
			}
		}
		return 10 * time.Second
	}()

	// sftpMaxConnsPerFile is the maximum number of concurrent connections to a file
	//
	// See: https://godoc.org/github.com/pkg/sftp#MaxConcurrentRequestsPerFile
	sftpMaxConnsPerFile = func() int {
		if n, err := strconv.Atoi(os.Getenv("SFTP_MAX_CONNS_PER_FILE")); err == nil {
			return n
		}
		return 8 // pkg/sftp's default is 64
	}()

	// sftpMaxPacketSize is the maximum size for each packet sent over SFTP.
	//
	// Their docs suggest lowering this on "failed to send packet header: EOF" errors,
	// so we're going to lower it by default (which is 32768).
	sftpMaxPacketSize = func() int {
		if n, err := strconv.Atoi(os.Getenv("SFTP_MAX_PACKET_SIZE")); err == nil {
			return n
		}
		return 20480
	}()
)

type SFTPConfig struct {
	RoutingNumber string

	Hostname string
	Username string

	Password         string
	ClientPrivateKey string

	HostPublicKey string
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
	agent := &SFTPTransferAgent{cfg: cfg, sftpConfigs: sftpConfigs}
	sftpConf := agent.findConfig()
	if sftpConf == nil {
		return nil, fmt.Errorf("sftp: unable to find config for %s", cfg.RoutingNumber)
	}

	// conn, _, _, err := sftpConnect(sftpConf)
	conn, stdin, stdout, err := sftpConnect(sftpConf)
	if err != nil {
		return nil, fmt.Errorf("filetransfer: %v", err)
	}
	agent.conn = conn

	// Setup our SFTP client
	var opts = []sftp.ClientOption{
		sftp.MaxConcurrentRequestsPerFile(sftpMaxConnsPerFile),
		sftp.MaxPacket(sftpMaxPacketSize),
	}
	// client, err := sftp.NewClient(conn, opts...)
	client, err := sftp.NewClientPipe(stdout, stdin, opts...)
	if err != nil {
		go conn.Close()
		return nil, fmt.Errorf("filetransfer: sftp connect: %v", err)
	}
	agent.client = client

	return agent, nil
}

func sftpConnect(sftpConf *SFTPConfig) (*ssh.Client, io.WriteCloser, io.Reader, error) {
	conf := &ssh.ClientConfig{
		User:            sftpConf.Username,
		Timeout:         sftpDialTimeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO(adam): insecure default, should fix
	}
	if sftpConf.HostPublicKey != "" {
		pubKey, err := ssh.ParsePublicKey([]byte(sftpConf.HostPublicKey)) // TODO(adam): attempt base64 decode
		if err != nil {
			return nil, nil, nil, fmt.Errorf("problem parsing ssh public key: %v", err)
		}
		conf.HostKeyCallback = ssh.FixedHostKey(pubKey)
	}
	switch {
	case sftpConf.Password != "":
		conf.Auth = append(conf.Auth, ssh.Password(sftpConf.Password))
	case sftpConf.ClientPrivateKey != "":
		signer, err := ssh.ParsePrivateKey([]byte(sftpConf.ClientPrivateKey)) // TODO(adam): attempt base64 decode also
		if err != nil {
			return nil, nil, nil, fmt.Errorf("sftpConnect: failed to read client private key: %v", err)
		}
		conf.Auth = append(conf.Auth, ssh.PublicKeys(signer))
	default:
		return nil, nil, nil, fmt.Errorf("sftpConnect: no auth method provided for routingNumber=%s", sftpConf.RoutingNumber)
	}

	// Connect to the remote server
	var client *ssh.Client
	var err error
	for i := 0; i < 3; i++ {
		if client == nil {
			client, err = ssh.Dial("tcp", sftpConf.Hostname, conf) // retry connection
			time.Sleep(250 * time.Millisecond)
		}
	}
	if client == nil && err != nil {
		return nil, nil, nil, fmt.Errorf("sftpConnect: error with routingNumber=%s: %v", sftpConf.RoutingNumber, err)
	}

	session, err := client.NewSession()
	if err != nil {
		go client.Close()
		return nil, nil, nil, err
	}
	if err = session.RequestSubsystem("sftp"); err != nil {
		go client.Close()
		return nil, nil, nil, err
	}
	pw, err := session.StdinPipe()
	if err != nil {
		go client.Close()
		return nil, nil, nil, err
	}
	pr, err := session.StdoutPipe()
	if err != nil {
		go client.Close()
		return nil, nil, nil, err
	}

	return client, pw, pr, nil
}

func (a *SFTPTransferAgent) Ping() error {
	_, err := a.client.ReadDir(".")
	if err != nil {
		return fmt.Errorf("sftp: ping %v", err)
	}
	return nil
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
		fd, err := agent.client.Open(filepath.Join(dir, infos[i].Name()))
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
