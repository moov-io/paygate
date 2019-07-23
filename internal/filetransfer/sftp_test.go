// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/moov-io/base/docker"

	"github.com/ory/dockertest"
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
		InboundPath:  "/upload/",
		OutboundPath: "/upload/",
		ReturnPath:   "/upload/",
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
	return newSFTPTransferAgent(cfg, sftpConfigs)
}

func TestSFTP(t *testing.T) {
	if testing.Short() {
		return
	}

	// This test server is available for lots of various protocols
	// and services. We can only list files as the server is read-only.
	// See: https://test.rebex.net/
	agent, err := newAgent("test.rebex.net:22", "demo", "password", "")
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	fds, err := agent.client.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for i := range fds {
		t.Logf(" #%d %s", i, fds[i].Name())
	}
	if len(fds) == 0 {
		t.Errorf("expected to find files, we've found them before")
	}

	if err := agent.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestSFTP__password(t *testing.T) {
	deployment := spawnSFTP(t)
	defer deployment.close(t)

	if err := deployment.agent.Ping(); err != nil {
		t.Fatal(err)
	}

	err := deployment.agent.UploadFile(File{
		Filename: deployment.agent.OutboundPath() + "upload.ach",
		Contents: ioutil.NopCloser(strings.NewReader("test data")),
	})
	if err != nil {
		t.Fatal(err)
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

func TestSFTP__ClientPrivateKey(t *testing.T) {

}

func TestSFTP__sftpConnect(t *testing.T) {
	client, _, _, err := sftpConnect(&SFTPConfig{
		Username: "foo",
	})
	if client != nil || err == nil {
		t.Errorf("client=%v err=%v", client, err)
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
