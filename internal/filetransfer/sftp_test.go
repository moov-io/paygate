// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	// "github.com/pkg/sftp"

	"github.com/moov-io/base/docker"

	"github.com/ory/dockertest"
)

type sftpDeployment struct {
	res   *dockertest.Resource
	agent *SFTPTransferAgent
}

func (s *sftpDeployment) close(t *testing.T) {
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
func spawnSFTP(t *testing.T, containerArgs string) *sftpDeployment {
	if testing.Short() {
		t.Skip("-short flag enabled")
	}
	if !docker.Enabled() {
		t.Skip("Docker not enabled")
	}

	// Start our Docker image
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatal(err)
	}
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "atmoz/sftp",
		Tag:        "latest",
		Cmd:        []string{"demo:password:::upload"},
	})
	if err != nil {
		t.Fatal(err)
	}
	addr := fmt.Sprintf("localhost:%s", resource.GetPort("22/tcp"))
	agent, err := newAgent(addr, "demo", "password", "")
	if err != nil {
		t.Fatal(err)
	}
	err = pool.Retry(func() error {
		return agent.Ping()
	})
	if err != nil {
		t.Fatal(err)
	}
	return &sftpDeployment{res: resource, agent: agent}
}

func newAgent(host, user, pass, passFile string) (*SFTPTransferAgent, error) {
	cfg := &Config{
		RoutingNumber: "121042882", // arbitrary routing number
		InboundPath:   "upload/inbound/",
		OutboundPath:  "upload/outbound/",
		ReturnPath:    "upload/returns/",
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

// docker run -p 22:22 -d atmoz/sftp foo:pass:::upload

func TestSFTP__password(t *testing.T) {
	t.Skip("can't connect to the Docker image for some reason...")

	deployment := spawnSFTP(t, "foo:pass:::upload")
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
