// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/base/docker"
	moovcustomers "github.com/moov-io/customers/pkg/client"
	"github.com/moov-io/paygate/pkg/config"

	"github.com/go-kit/kit/log"
	"github.com/ory/dockertest/v3"
)

type customersDeployment struct {
	watchman  *dockertest.Resource
	customers *dockertest.Resource

	client     Client
	underlying *moovcustomers.APIClient
}

func (d *customersDeployment) close(t *testing.T) {
	if err := d.watchman.Close(); err != nil {
		t.Error(err)
	}
	if err := d.customers.Close(); err != nil {
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

	watchmanContainer, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "moov/watchman",
		Tag:        "static",
		Cmd:        []string{"-http.addr=:8080"},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = pool.Retry(func() error {
		resp, err := http.DefaultClient.Get(fmt.Sprintf("http://localhost:%s/ping", watchmanContainer.GetPort("8080/tcp")))
		if err != nil {
			return err
		}
		return resp.Body.Close()
	})
	if err != nil {
		t.Fatal(err)
	}

	customersContainer, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "moov/customers",
		Tag:        "v0.4.0-rc3",
		Cmd:        []string{"-http.addr=:8080"},
		Links:      []string{fmt.Sprintf("%s:watchman", watchmanContainer.Container.Name)},
		Env:        []string{"WATCHMAN_ENDPOINT=http://watchman:8080"},
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Customers{
		Endpoint: fmt.Sprintf("http://localhost:%s", customersContainer.GetPort("8080/tcp")),
	}
	client := NewClient(log.NewNopLogger(), cfg, nil)
	err = pool.Retry(func() error {
		return client.Ping()
	})
	if err != nil {
		t.Fatal(err)
	}
	deployment := &customersDeployment{
		watchman:  watchmanContainer,
		customers: customersContainer,
		client:    client,
	}
	if c, ok := client.(*moovClient); ok {
		deployment.underlying = c.underlying
	}
	return deployment
}

func TestCustomers__client(t *testing.T) {
	cfg := config.Customers{
		Endpoint: "",
	}
	if client := NewClient(log.NewNopLogger(), cfg, nil); client == nil {
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

	cust := createCustomer(t, deployment)
	cust, err := deployment.client.Lookup(cust.CustomerID, base.ID(), base.ID())
	if err != nil {
		t.Fatal(err)
	}
	if cust == nil || cust.CustomerID == "" {
		t.Fatal("nil Customer")
	}

	deployment.close(t) // close only if successful
}

func TestCustomers__OFACSearch(t *testing.T) {
	deployment := spawnCustomers(t)

	if err := deployment.client.Ping(); err != nil {
		t.Fatal(err)
	}

	cust := createCustomer(t, deployment)

	_, err := deployment.client.LatestOFACSearch(cust.CustomerID, "requestID", "userID")
	if err != nil {
		t.Fatal(err)
	}

	result, err := deployment.client.RefreshOFACSearch(cust.CustomerID, "requestID", "userID")
	if err != nil {
		t.Fatal(err)
	}
	if result.EntityId == "" || result.Match < 0.01 {
		t.Errorf("result=%#v", result)
	}

	deployment.close(t)
}

func createCustomer(t *testing.T, deployment *customersDeployment) *moovcustomers.Customer {
	req := moovcustomers.CreateCustomer{
		FirstName: "Jane",
		LastName:  "Doe",
		Email:     "jane.doe@moov.io",
	}
	cust, resp, err := deployment.underlying.CustomersApi.CreateCustomer(context.Background(), req, nil)
	if resp != nil {
		resp.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	return &cust
}
