// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/paygate/internal/config"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPConfig struct {
	RoutingNumber    string `yaml:"routingNumber"`
	Hostname         string `yaml:"hostname"`
	Username         string `yaml:"username"`
	Password         string `yaml:"password"`
	ClientPrivateKey string `yaml:"clientPrivateKey"`
	HostPublicKey    string `yaml:"hostPublicKey"`
}

func (cfg *SFTPConfig) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("SFTPConfig{RoutingNumber=%s, ", cfg.RoutingNumber))
	buf.WriteString(fmt.Sprintf("Hostname=%s, ", cfg.Hostname))
	buf.WriteString(fmt.Sprintf("Username=%s, ", cfg.Username))
	buf.WriteString(fmt.Sprintf("Password=%s, ", maskPassword(cfg.Password)))
	buf.WriteString(fmt.Sprintf("ClientPrivateKey:%v, ", cfg.ClientPrivateKey != ""))
	buf.WriteString(fmt.Sprintf("HostPublicKey:%v}, ", cfg.HostPublicKey != ""))
	return buf.String()
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

func newSFTPTransferAgent(logger log.Logger, config *config.Config, transferConfig *Config, sftpConfigs []*SFTPConfig) (*SFTPTransferAgent, error) {
	agent := &SFTPTransferAgent{cfg: transferConfig, sftpConfigs: sftpConfigs}
	sftpConf := agent.findConfig()
	if sftpConf == nil {
		return nil, fmt.Errorf("sftp: unable to find config for %s", transferConfig.RoutingNumber)
	}

	conn, stdin, stdout, err := sftpConnect(logger, config.SFTP, sftpConf)
	if err != nil {
		return nil, fmt.Errorf("filetransfer: %v", err)
	}
	agent.conn = conn

	// Setup our SFTP client
	var opts = []sftp.ClientOption{
		sftp.MaxConcurrentRequestsPerFile(sftpMaxConnsPerFile(config.SFTP)),
		sftp.MaxPacket(sftpMaxPacketSize(config.SFTP)),
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

func sftpDialTimeout(cfg *config.SFTPConfig) time.Duration {
	if cfg == nil || cfg.DialTimeout == 0 {
		return 10 * time.Second
	}
	return cfg.DialTimeout
}

// sftpMaxConnsPerFile is the maximum number of concurrent connections to a file
//
// See: https://godoc.org/github.com/pkg/sftp#MaxConcurrentRequestsPerFile
func sftpMaxConnsPerFile(cfg *config.SFTPConfig) int {
	if cfg == nil || cfg.MaxConnectionsPerFile == 0 {
		return 8 // pkg/sftp's default is 64
	}
	return cfg.MaxConnectionsPerFile
}

// sftpMaxPacketSize is the maximum size for each packet sent over SFTP.
//
// Their docs suggest lowering this on "failed to send packet header: EOF" errors,
// so we're going to lower it by default (which is 32768).
func sftpMaxPacketSize(cfg *config.SFTPConfig) int {
	if cfg == nil || cfg.MaxPacketSize == 0 {
		return 20480
	}
	return cfg.MaxPacketSize
}

var (
	hostKeyCallbackOnce sync.Once
	hostKeyCallback     = func(logger log.Logger) {
		logger.Log("sftp", "WARNING!!! Insecure default of skipping SFTP host key validation. Please set sftp_configs.host_public_key")
	}
)

func sftpConnect(logger log.Logger, config *config.SFTPConfig, sftpConf *SFTPConfig) (*ssh.Client, io.WriteCloser, io.Reader, error) {
	conf := &ssh.ClientConfig{
		User:    sftpConf.Username,
		Timeout: sftpDialTimeout(config),
	}
	conf.SetDefaults()

	if sftpConf.HostPublicKey != "" {
		pubKey, err := readPubKey(sftpConf.HostPublicKey)
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
	case sftpConf.Password != "":
		conf.Auth = append(conf.Auth, ssh.Password(sftpConf.Password))
	case sftpConf.ClientPrivateKey != "":
		signer, err := readSigner(sftpConf.ClientPrivateKey)
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

	// Create OutboundPath if it doesn't exist
	info, err := agent.client.Stat(agent.cfg.OutboundPath)
	if info == nil || (err != nil && os.IsNotExist(err)) {
		if err := agent.client.Mkdir(agent.cfg.OutboundPath); err != nil {
			return fmt.Errorf("sft: problem creating parent dir %s: %v", agent.cfg.OutboundPath, err)
		}
	}

	// Take the base of f.Filename and our (out of band) OutboundPath to avoid accepting a write like '../../../../etc/passwd'.
	fd, err := agent.client.Create(filepath.Join(agent.cfg.OutboundPath, filepath.Base(f.Filename)))
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
			if err != nil && !strings.Contains(err.Error(), sftp.InternalInconsistency.Error()) {
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
