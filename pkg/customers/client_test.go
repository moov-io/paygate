// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/antihax/optional"
	"github.com/moov-io/base/docker"
	moovcustomers "github.com/moov-io/customers/pkg/client"

	"github.com/moov-io/paygate/pkg/config"

	"github.com/moov-io/base/log"
	"github.com/ory/dockertest/v3"
)

type customersDeployment struct {
	watchman  *dockertest.Resource
	customers *dockertest.Resource

	client     Client
	underlying *moovcustomers.APIClient
}

func (d *customersDeployment) close(t *testing.T) {
	if d.watchman != nil {
		if err := d.watchman.Close(); err != nil {
			t.Error(err)
		}
		d.watchman = nil
	}
	if d.customers != nil {
		if err := d.customers.Close(); err != nil {
			t.Error(err)
		}
		d.customers = nil
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

	network, err := pool.CreateNetwork(fmt.Sprintf("paygate-customers-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = network.Close() })

	// moov/watchman:static ships frozen list data and listens on :8084.
	// Passing -http.addr=:8080 (the old v1 flag) is ignored by current images,
	// and 8080 is no longer EXPOSEd, so GetPort("8080/tcp") was empty.
	watchmanContainer, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository:   "moov/watchman",
		Tag:          "static",
		Hostname:     "watchman",
		Networks:     []*dockertest.Network{network},
		ExposedPorts: []string{"8084/tcp"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = watchmanContainer.Close() })

	watchmanPort := watchmanContainer.GetPort("8084/tcp")
	if watchmanPort == "" {
		t.Fatal("watchman container did not publish 8084/tcp")
	}

	err = pool.Retry(func() error {
		resp, err := http.DefaultClient.Get(fmt.Sprintf("http://127.0.0.1:%s/ping", watchmanPort))
		if err != nil {
			return err
		}
		return resp.Body.Close()
	})
	if err != nil {
		t.Fatal(err)
	}

	customersContainer, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository:   "moov/customers",
		Tag:          "v0.5.0-dev25",
		Cmd:          []string{"-http.addr=:8080"},
		ExposedPorts: []string{"8080/tcp"},
		Networks:     []*dockertest.Network{network},
		Env:          []string{"WATCHMAN_ENDPOINT=http://watchman:8084"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = customersContainer.Close() })

	customersPort := customersContainer.GetPort("8080/tcp")
	if customersPort == "" {
		t.Fatal("customers container did not publish 8080/tcp")
	}

	cfg := config.Customers{
		Debug:    testing.Verbose(),
		Endpoint: fmt.Sprintf("http://127.0.0.1:%s", customersPort),
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
	cust, err := deployment.client.Lookup("moov", cust.CustomerID, "requestID")
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

	_, err := deployment.client.LatestOFACSearch("moov", cust.CustomerID, "requestID")
	if err != nil {
		t.Fatal(err)
	}

	result, err := deployment.client.RefreshOFACSearch("moov", cust.CustomerID, "requestID")
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
		Type:      moovcustomers.CUSTOMERTYPE_INDIVIDUAL,
	}
	opts := &moovcustomers.CreateCustomerOpts{
		XOrganization: optional.NewString("moov"),
	}
	cust, resp, err := deployment.underlying.CustomersApi.CreateCustomer(context.Background(), req, opts)
	if resp != nil {
		resp.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	return &cust
}
