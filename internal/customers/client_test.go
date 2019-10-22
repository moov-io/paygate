// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"fmt"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/base/docker"
	"github.com/ory/dockertest"
)

type customersDeployment struct {
	res    *dockertest.Resource
	client Client
}

func (d *customersDeployment) close(t *testing.T) {
	if err := d.res.Close(); err != nil {
		t.Error(err)
	}
}

func spawnCustomers(t *testing.T) *customersDeployment {
	// no t.Helper() call so we know where it failed

	if testing.Short() {
		t.Skip("-short flag enabled")
	}
	if !docker.Enabled() {
		t.Skip("Docker not enabled")
	}

	// Spawn Customers docker image
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatal(err)
	}
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "moov/customers",
		Tag:        "v0.3.0-dev",
		Cmd:        []string{"-http.addr=:8080"},
	})
	if err != nil {
		t.Fatal(err)
	}

	addr := fmt.Sprintf("http://localhost:%s", resource.GetPort("8080/tcp"))
	client := NewClient(log.NewNopLogger(), addr, nil)
	err = pool.Retry(func() error {
		return client.Ping()
	})
	if err != nil {
		t.Fatal(err)
	}
	return &customersDeployment{resource, client}
}

func TestCustomers__client(t *testing.T) {
	endpoint := ""
	if client := NewClient(log.NewNopLogger(), endpoint, nil); client == nil {
		t.Fatal("expected non-nil client")
	}

	// Spawn an Customers Docker image and ping against it
	deployment := spawnCustomers(t)
	if err := deployment.client.Ping(); err != nil {
		t.Fatal(err)
	}
	deployment.close(t) // close only if successful
}
