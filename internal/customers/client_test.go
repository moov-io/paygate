// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/base"
	"github.com/moov-io/base/docker"
	moovcustomers "github.com/moov-io/customers/client"
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

func TestCustomers(t *testing.T) {
	deployment := spawnCustomers(t)

	if err := deployment.client.Ping(); err != nil {
		t.Fatal(err)
	}

	cust, err := deployment.client.Create(&Request{
		Name:  "John Smith",
		Email: "john.smith@moov.io",
		SSN:   "12314567",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cust == nil || cust.ID == "" {
		t.Fatal("nil Customer")
	}

	cust, err = deployment.client.Lookup(cust.ID, base.ID(), base.ID())
	if err != nil {
		t.Fatal(err)
	}
	if cust == nil || cust.ID == "" {
		t.Fatal("nil Customer")
	}

	deployment.close(t) // close only if successful
}

func TestCustomers__disclaimers(t *testing.T) {
	deployment := spawnCustomers(t)

	if err := deployment.client.Ping(); err != nil {
		t.Fatal(err)
	}

	// TODO(adam): So, we don't have a way to expose the admin port
	// (which is needed to actually create a disclaimer).

	customerID := base.ID()
	disclaimers, err := deployment.client.GetDisclaimers(customerID, base.ID(), base.ID())
	if err != nil {
		t.Fatal(err)
	}
	if n := len(disclaimers); n != 0 {
		t.Errorf("got %d disclaimers", n)
	}

	if err := HasAcceptedAllDisclaimers(deployment.client, customerID, base.ID(), base.ID()); err != nil {
		t.Errorf("expected no error: %v", err)
	}

	deployment.close(t) // close only if successful
}

func TestCustomers__hasAcceptedAllDisclaimers(t *testing.T) {
	client := &TestClient{
		Disclaimers: []moovcustomers.Disclaimer{
			{
				ID:   base.ID(),
				Text: "requirements",
			},
		},
	}
	customerID := base.ID()

	if err := HasAcceptedAllDisclaimers(client, customerID, base.ID(), base.ID()); err == nil {
		t.Error("expected error (unaccepted disclaimer)")
	}

	client.Disclaimers[0].AcceptedAt = time.Now()
	if err := HasAcceptedAllDisclaimers(client, customerID, base.ID(), base.ID()); err != nil {
		t.Errorf("expected no error: %v", err)
	}

	client.Err = errors.New("bad error")
	if err := HasAcceptedAllDisclaimers(client, customerID, base.ID(), base.ID()); err == nil {
		t.Error("expeced error")
	}
}
