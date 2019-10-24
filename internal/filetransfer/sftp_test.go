// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/moov-io/base/docker"
	"github.com/moov-io/paygate/internal/config"

	"github.com/go-kit/kit/log"
	"github.com/ory/dockertest/v3"
)

type sftpDeployment struct {
	res   *dockertest.Resource
	agent *SFTPTransferAgent

	dir string // temporary directory
}

func (s *sftpDeployment) close(t *testing.T) {
	defer func() {
		// Always try and cleanup our scratch dir
		if err := os.RemoveAll(s.dir); err != nil {
			t.Error(err)
		}
	}()

	if err := s.agent.Close(); err != nil {
		t.Error(err)
	}
	if err := s.res.Close(); err != nil {
		t.Error(err)
	}
}

// spawnSFTP launches an SFTP Docker image
//
// You can verify this container launches with an ssh command like:
//  $ ssh ssh://demo@127.0.0.1:33138 -s sftp
func spawnSFTP(t *testing.T) *sftpDeployment {
	if testing.Short() {
		t.Skip("-short flag enabled")
	}
	if !docker.Enabled() {
		t.Skip("Docker not enabled")
	}
	switch runtime.GOOS {
	case "darwin", "linux":
		// continue on with our test
	default:
		t.Skipf("we haven't coded test support for uid/gid extraction on %s", runtime.GOOS)
	}

	// Setup a temp directory for our SFTP instance
	dir, uid, gid := mkdir(t)

	// Start our Docker image
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatal(err)
	}
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "atmoz/sftp",
		Tag:        "latest",
		// set user and group to grant write permissions
		Cmd: []string{
			fmt.Sprintf("demo:password:%d:%d:upload", uid, gid),
		},
		Mounts: []string{
			fmt.Sprintf("%s:/home/demo/upload", dir),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	addr := fmt.Sprintf("localhost:%s", resource.GetPort("22/tcp"))

	var agent *SFTPTransferAgent
	for i := 0; i < 10; i++ {
		if agent == nil {
			agent, err = newAgent(addr, "demo", "password", "")
			time.Sleep(250 * time.Millisecond)
		}
	}
	if agent == nil && err != nil {
		t.Fatal(err)
	}
	err = pool.Retry(func() error {
		return agent.Ping()
	})
	if err != nil {
		t.Fatal(err)
	}
	return &sftpDeployment{res: resource, agent: agent, dir: dir}
}

func mkdir(t *testing.T) (string, uint32, uint32) {
	wd, _ := os.Getwd()
	dir, err := ioutil.TempDir(wd, "sftp")
	if err != nil {
		t.Fatal(err)
	}
	fd, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	stat, ok := fd.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("unable to stat %s", fd.Name())
	}
	return dir, stat.Uid, stat.Gid
}

func newAgent(host, user, pass, passFile string) (*SFTPTransferAgent, error) {
	cfg := &Config{
		RoutingNumber: "121042882", // arbitrary routing number
		// Our SFTP client inits into '/' with one folder, 'upload', so we need to
		// put files into /upload/ (as an absolute path).
		//
		// Currently it's assumed sub-directories would exist for inbound vs outbound files.
		InboundPath:  "/upload/inbound/",
		OutboundPath: "/upload/",
		ReturnPath:   "/upload/returned/",
	}
	sftpConfigs := []*SFTPConfig{
		{
			RoutingNumber: "121042882",
			Hostname:      host,
			Username:      user,
		},
	}
	if pass != "" {
		sftpConfigs[0].Password = pass
	} else {
		sftpConfigs[0].ClientPrivateKey = passFile
	}
	return newSFTPTransferAgent(log.NewNopLogger(), config.Empty(), cfg, sftpConfigs)
}

func cp(from, to string) error {
	f, err := os.Open(from)
	if err != nil {
		return err
	}
	t, err := os.Create(to)
	if err != nil {
		return err
	}
	_, err = io.Copy(t, f)
	return err
}

func TestSFTP__password(t *testing.T) {
	deployment := spawnSFTP(t)
	defer deployment.close(t)

	if err := deployment.agent.Ping(); err != nil {
		t.Fatal(err)
	}

	err := deployment.agent.UploadFile(File{
		Filename: "upload.ach",
		Contents: ioutil.NopCloser(strings.NewReader("test data")),
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := deployment.agent.Delete(deployment.agent.OutboundPath() + "upload.ach"); err != nil {
		t.Fatal(err)
	}

	// Inbound files (IAT in our testdata/sftp-server/)
	os.MkdirAll(filepath.Join(deployment.dir, "inbound"), 0777)
	err = cp(
		filepath.Join("..", "..", "testdata", "sftp-server", "inbound", "iat-credit.ach"),
		filepath.Join(deployment.dir, "inbound", "iat-credit.ach"),
	)
	if err != nil {
		t.Fatal(err)
	}
	files, err := deployment.agent.GetInboundFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Filename != "iat-credit.ach" {
		t.Errorf("%d of files: %#v", len(files), files)
	}

	// Return files (WEB in our testdata/sftp-server/)
	os.MkdirAll(filepath.Join(deployment.dir, "returned"), 0777)
	err = cp(
		filepath.Join("..", "..", "testdata", "sftp-server", "returned", "return-WEB.ach"),
		filepath.Join(deployment.dir, "returned", "return-WEB.ach"),
	)
	if err != nil {
		t.Fatal(err)
	}
	files, err = deployment.agent.GetReturnFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Filename != "return-WEB.ach" {
		t.Errorf("%d of files: %#v", len(files), files)
	}
}

// TestSFTP__readFilesEmpty is setup to encounter error cases with readFiles
func TestSFTP__readFilesEmpty(t *testing.T) {
	deployment := spawnSFTP(t)
	defer deployment.close(t)

	if err := deployment.agent.Ping(); err != nil {
		t.Fatal(err)
	}

	// Upload an empty file
	err := deployment.agent.UploadFile(File{
		Filename: "upload.ach",
		Contents: ioutil.NopCloser(strings.NewReader("")),
	})
	if err == nil || !strings.Contains(err.Error(), "sftp: problem copying (n=0) upload.ach") {
		t.Fatal(err)
	}

	path := filepath.Join(deployment.agent.OutboundPath(), "upload.ach")

	// Truncate and then copy down
	if err := deployment.agent.client.Truncate(path, 0); err != nil {
		t.Fatal(err)
	}
	info, err := deployment.agent.client.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := info.Size(); n != 0 {
		t.Errorf("upload.ach is %d bytes", n)
	}

	// Read the empty file
	files, err := deployment.agent.readFiles(deployment.agent.OutboundPath())
	if err == nil || !strings.Contains(err.Error(), "sftp: read (n=0) on upload.ach") {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("files: %#v", files)
	}

	// read a non-existent directory
	files, err = deployment.agent.readFiles("/dev/null")
	if err == nil {
		t.Errorf("expected error -- files: %#v", files)
	}
}

// Generate keys (in Go) and mount them into our test container
//
// docker run \
//     -v /host/id_rsa.pub:/home/foo/.ssh/keys/id_rsa.pub:ro \
//     -v /host/id_other.pub:/home/foo/.ssh/keys/id_other.pub:ro \
//     -v /host/share:/home/foo/share \
//     -p 2222:22 -d atmoz/sftp \
//     foo::1001

func TestSFTP__ClientPrivateKey(t *testing.T) { // TODO(adam): need to write this test

}

func TestSFTP__uploadFile(t *testing.T) {
	deployment := spawnSFTP(t)
	defer deployment.close(t)

	if err := deployment.agent.Ping(); err != nil {
		t.Fatal(err)
	}

	// force out OutboundPath to create more directories
	deployment.agent.cfg.OutboundPath = filepath.Join("upload", "foo")
	err := deployment.agent.UploadFile(File{
		Filename: "upload.ach",
		Contents: ioutil.NopCloser(strings.NewReader("test data")),
	})
	if err != nil {
		t.Fatal(err)
	}

	// fail to create the OutboundPath
	deployment.agent.cfg.OutboundPath = string(os.PathSeparator) + filepath.Join("home", "bad-path")
	err = deployment.agent.UploadFile(File{
		Filename: "upload.ach",
		Contents: ioutil.NopCloser(strings.NewReader("test data")),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSFTP__readPubKey(t *testing.T) {
	// TODO(adam): test with '-----BEGIN RSA PRIVATE KEY-----' PKCS#8 format

	// Generated with 'ssh-keygen -t rsa -b 2048 -f test'
	raw := `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDts86Xot/AZN9kWO5bJNukRv669fOjtsKiaAcjoG2NNsUjQuEcetg5ghhzdEtDUkeLYFAJ4H7rFHcYxBS4HSzJd6qhO5WgtGyTSwLmo/wi6736I/kMsxHlBJ67vh39oheYARDQrXTpVgmMOJ2xWhQWQaGFwF93Tp8TwxAs+cx9lmW6Z+52wCovj6HcJSEBXIIHDNL5wSqWwEUYtmAyqQ5mcDmQN7MJbBqzSq6fFSHdarS4XBOMb+BV27MANHpOHGrum8UbI1CfFfem+a7ln+HVMUBp5czovUW8Q/V8+zT57TQXwEro6obyVPC4HWiIDagywypgmwwW95ttP3pdwb7Z adam@Adams-MacBook-Pro.local`
	pubKey, err := readPubKey(raw)
	if pubKey == nil || err != nil {
		t.Fatalf("PublicKey=%v error=%v", pubKey, err)
	}

	// base64 Encoded
	raw = base64.StdEncoding.EncodeToString([]byte(raw))
	pubKey, err = readPubKey(raw)
	if pubKey == nil || err != nil {
		t.Fatalf("PublicKey=%v error=%v", pubKey, err)
	}
}

func TestSFTP__readSigner(t *testing.T) {
	raw := `-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQCxoeCUW5KJxNPxMp+KmCxKLc1Zv9Ny+4CFqcUXVUYH69L3mQ7v
IWrJ9GBfcaA7BPQqUlWxWM+OCEQZH1EZNIuqRMNQVuIGCbz5UQ8w6tS0gcgdeGX7
J7jgCQ4RK3F/PuCM38QBLaHx988qG8NMc6VKErBjctCXFHQt14lerd5KpQIDAQAB
AoGAYrf6Hbk+mT5AI33k2Jt1kcweodBP7UkExkPxeuQzRVe0KVJw0EkcFhywKpr1
V5eLMrILWcJnpyHE5slWwtFHBG6a5fLaNtsBBtcAIfqTQ0Vfj5c6SzVaJv0Z5rOd
7gQF6isy3t3w9IF3We9wXQKzT6q5ypPGdm6fciKQ8RnzREkCQQDZwppKATqQ41/R
vhSj90fFifrGE6aVKC1hgSpxGQa4oIdsYYHwMzyhBmWW9Xv/R+fPyr8ZwPxp2c12
33QwOLPLAkEA0NNUb+z4ebVVHyvSwF5jhfJxigim+s49KuzJ1+A2RaSApGyBZiwS
rWvWkB471POAKUYt5ykIWVZ83zcceQiNTwJBAMJUFQZX5GDqWFc/zwGoKkeR49Yi
MTXIvf7Wmv6E++eFcnT461FlGAUHRV+bQQXGsItR/opIG7mGogIkVXa3E1MCQARX
AAA7eoZ9AEHflUeuLn9QJI/r0hyQQLEtrpwv6rDT1GCWaLII5HJ6NUFVf4TTcqxo
6vdM4QGKTJoO+SaCyP0CQFdpcxSAuzpFcKv0IlJ8XzS/cy+mweCMwyJ1PFEc4FX6
wg/HcAJWY60xZTJDFN+Qfx8ZQvBEin6c2/h+zZi5IVY=
-----END RSA PRIVATE KEY-----`

	sig, err := readSigner(raw)
	if sig == nil || err != nil {
		t.Fatalf("Signer=%v error=%v", sig, err)
	}

	// base64 Encoded
	raw = base64.StdEncoding.EncodeToString([]byte(raw))
	sig, err = readSigner(raw)
	if sig == nil || err != nil {
		t.Fatalf("Signer=%v error=%v", sig, err)
	}
}

func TestSFTP__sftpConnect(t *testing.T) {
	cfg := config.Empty()

	client, _, _, err := sftpConnect(log.NewNopLogger(), cfg.SFTP, &SFTPConfig{
		Username: "foo",
	})
	if client != nil || err == nil {
		t.Errorf("client=%v err=%v", client, err)
	}

	// bad host public key
	_, _, _, err = sftpConnect(log.NewNopLogger(), cfg.SFTP, &SFTPConfig{
		HostPublicKey: "bad key material",
	})
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestSFTPAgent(t *testing.T) {
	agent := &SFTPTransferAgent{
		cfg: &Config{
			InboundPath: "inbound",
		},
	}
	if v := agent.InboundPath(); v != "inbound" {
		t.Errorf("agent.InboundPath()=%s", agent.InboundPath())
	}

	agent.cfg.ReturnPath = "return"
	if v := agent.ReturnPath(); v != "return" {
		t.Errorf("agent.ReturnPath()=%s", agent.ReturnPath())
	}
}

func TestSFTPAgent__findConfig(t *testing.T) {
	agent := &SFTPTransferAgent{
		cfg: &Config{
			RoutingNumber: "987654320",
		},
	}
	if conf := agent.findConfig(); conf != nil {
		t.Error("expected nil")
	}
}

func TestSFTPConfig__String(t *testing.T) {
	cfg := &SFTPConfig{"routing", "host", "user", "pass", "clientPriv", "hostPub"}
	if !strings.Contains(cfg.String(), "Password=p**s") {
		t.Error(cfg.String())
	}
}
