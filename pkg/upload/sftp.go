// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package upload

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/paygate/pkg/config"
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

type SFTPTransferAgent struct {
	conn   *ssh.Client
	client *sftp.Client
	cfg    config.ODFI
	logger log.Logger
	mu     sync.Mutex // protects all read/write methods
}

func newSFTPTransferAgent(logger log.Logger, cfg config.ODFI) (*SFTPTransferAgent, error) {
	agent := &SFTPTransferAgent{cfg: cfg, logger: logger}

	if err := rejectOutboundIPRange(cfg.SplitAllowedIPs(), cfg.SFTP.Hostname); err != nil {
		return nil, fmt.Errorf("sftp: %s is not whitelisted: %v", cfg.SFTP.Hostname, err)
	}

	_, err := agent.connection()

	return agent, err
}

// connection returns an sftp.Client which is connected to the remote server.
// This function will attempt to establish a new connection if none exists already.
//
// connection must be called within a mutex lock.
func (agent *SFTPTransferAgent) connection() (*sftp.Client, error) {
	if agent == nil || agent.cfg.SFTP == nil {
		return nil, errors.New("nil agent / config")
	}

	if agent.client != nil {
		// Verify the connection works and if not drop through and reconnect
		if _, err := agent.client.Getwd(); err == nil {
			return agent.client, nil
		} else {
			// Our connection is having issues, so retry connecting
			agent.client.Close()
		}
	}

	conn, stdin, stdout, err := sftpConnect(agent.logger, agent.cfg)
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

	return agent.client, nil
}

var (
	hostKeyCallbackOnce sync.Once
	hostKeyCallback     = func(logger log.Logger) {
		logger.Log("sftp", "WARNING!!! Insecure default of skipping SFTP host key validation. Please set sftp_configs.host_public_key")
	}
)

func sftpConnect(logger log.Logger, cfg config.ODFI) (*ssh.Client, io.WriteCloser, io.Reader, error) {
	if cfg.SFTP == nil {
		return nil, nil, nil, errors.New("nil config or sftp config")
	}

	conf := &ssh.ClientConfig{
		User:    cfg.SFTP.Username,
		Timeout: sftpDialTimeout,
	}
	conf.SetDefaults()

	if cfg.SFTP.HostPublicKey != "" {
		pubKey, err := readPubKey(cfg.SFTP.HostPublicKey)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("problem parsing ssh public key: %v", err)
		}
		conf.HostKeyCallback = ssh.FixedHostKey(pubKey)
	} else {
		hostKeyCallbackOnce.Do(func() {
			hostKeyCallback(logger)
		})
		conf.HostKeyCallback = ssh.InsecureIgnoreHostKey() // insecure default
	}
	switch {
	case cfg.SFTP.Password != "":
		conf.Auth = append(conf.Auth, ssh.Password(cfg.SFTP.Password))
	case cfg.SFTP.ClientPrivateKey != "":
		signer, err := readSigner(cfg.SFTP.ClientPrivateKey)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("sftpConnect: failed to read client private key: %v", err)
		}
		conf.Auth = append(conf.Auth, ssh.PublicKeys(signer))
	default:
		return nil, nil, nil, fmt.Errorf("sftpConnect: no auth method provided for routingNumber=%s", cfg.RoutingNumber)
	}

	// Connect to the remote server
	var client *ssh.Client
	var err error
	for i := 0; i < 3; i++ {
		if client == nil {
			client, err = ssh.Dial("tcp", cfg.SFTP.Hostname, conf) // retry connection
			time.Sleep(250 * time.Millisecond)
		}
	}
	if client == nil && err != nil {
		return nil, nil, nil, fmt.Errorf("sftpConnect: error with routingNumber=%s: %v", cfg.RoutingNumber, err)
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

func readPubKey(raw string) (ssh.PublicKey, error) {
	readAuthd := func(raw string) (ssh.PublicKey, error) {
		pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(raw))
		return pub, err
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if len(decoded) > 0 && err == nil {
		if pub, err := readAuthd(string(decoded)); pub != nil && err == nil {
			return pub, nil
		}
		return ssh.ParsePublicKey(decoded)
	}

	if pub, err := readAuthd(raw); pub != nil && err == nil {
		return pub, nil
	}
	return ssh.ParsePublicKey([]byte(raw))
}

func readSigner(raw string) (ssh.Signer, error) {
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if len(decoded) > 0 && err == nil {
		return ssh.ParsePrivateKey(decoded)
	}
	return ssh.ParsePrivateKey([]byte(raw))
}

func (agent *SFTPTransferAgent) Ping() error {
	if agent == nil {
		return errors.New("nil SFTPTransferAgent")
	}

	agent.mu.Lock()
	defer agent.mu.Unlock()

	conn, err := agent.connection()
	if err != nil {
		return err
	}

	if _, err := conn.ReadDir("."); err != nil {
		return fmt.Errorf("sftp: ping %v", err)
	}
	return nil
}

func (agent *SFTPTransferAgent) Close() error {
	if agent == nil {
		return nil
	}
	if agent.client != nil {
		agent.client.Close()
	}
	if agent.conn != nil {
		agent.conn.Close()
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
	agent.mu.Lock()
	defer agent.mu.Unlock()

	conn, err := agent.connection()
	if err != nil {
		return err
	}

	info, err := conn.Stat(path)
	if err != nil {
		return fmt.Errorf("sftp: delete stat: %v", err)
	}
	if info != nil {
		if err := conn.Remove(path); err != nil {
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

	conn, err := agent.connection()
	if err != nil {
		return err
	}

	// Create OutboundPath if it doesn't exist
	info, err := conn.Stat(agent.cfg.OutboundPath)
	if info == nil || (err != nil && os.IsNotExist(err)) {
		if err := conn.Mkdir(agent.cfg.OutboundPath); err != nil {
			return fmt.Errorf("sft: problem creating parent dir %s: %v", agent.cfg.OutboundPath, err)
		}
	}

	// Take the base of f.Filename and our (out of band) OutboundPath to avoid accepting a write like '../../../../etc/passwd'.
	fd, err := conn.Create(filepath.Join(agent.cfg.OutboundPath, filepath.Base(f.Filename)))
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
	agent.mu.Lock()
	defer agent.mu.Unlock()

	conn, err := agent.connection()
	if err != nil {
		return nil, err
	}

	infos, err := conn.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("sftp: readdir %s: %v", dir, err)
	}

	var files []File
	for i := range infos {
		fd, err := conn.Open(filepath.Join(dir, infos[i].Name()))
		if err != nil {
			return nil, fmt.Errorf("sftp: open %s: %v", infos[i].Name(), err)
		}
		var buf bytes.Buffer
		if n, err := io.Copy(&buf, fd); n == 0 || err != nil {
			fd.Close()
			if err != nil && !strings.Contains(err.Error(), sftp.ErrInternalInconsistency.Error()) {
				return nil, fmt.Errorf("sftp: read (n=%d) %s: %v", n, infos[i].Name(), err)
			}
			return nil, fmt.Errorf("sftp: read (n=%d) on %s", n, infos[i].Name())
		} else {
			fd.Close()
		}
		files = append(files, File{
			Filename: infos[i].Name(),
			Contents: ioutil.NopCloser(&buf),
		})
	}
	return files, nil
}
