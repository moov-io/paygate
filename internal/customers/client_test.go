// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/base"
	"github.com/moov-io/base/docker"
	"github.com/moov-io/paygate/pkg/id"
	"github.com/ory/dockertest/v3"
)

type customersDeployment struct {
	watchman  *dockertest.Resource
	customers *dockertest.Resource
	client    Client
}

func (d *customersDeployment) close(t *testing.T) {
	if err := d.watchman.Close(); err != nil {
		t.Error(err)
	}
	if err := d.customers.Close(); err != nil {
		t.Error(err)
	}
}

func (d *customersDeployment) adminPort() string {
	return d.customers.GetPort("9090/tcp")
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
		Tag:        "v0.13.2",
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
		Tag:        "v0.4.0-rc1",
		Cmd:        []string{"-http.addr=:8080", "-admin.addr=:9090"},
		Links:      []string{fmt.Sprintf("%s:watchman", watchmanContainer.Container.Name)},
		Env:        []string{"WATCHMAN_ENDPOINT=http://watchman:8080"},
	})
	if err != nil {
		t.Fatal(err)
	}

	addr := fmt.Sprintf("http://localhost:%s", customersContainer.GetPort("8080/tcp"))
	client := NewClient(log.NewNopLogger(), addr, nil)
	err = pool.Retry(func() error {
		return client.Ping()
	})
	if err != nil {
		t.Fatal(err)
	}
	return &customersDeployment{
		watchman:  watchmanContainer,
		customers: customersContainer,
		client:    client,
	}
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

	cust, err = deployment.client.Lookup(cust.ID, base.ID(), id.User(base.ID()))
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

	customerID := base.ID()

	address := fmt.Sprintf("http://localhost:%s/customers/%s/disclaimers", deployment.adminPort(), customerID)
	body := strings.NewReader(`{"text": "terms and conditions"}`)

	resp, err := http.DefaultClient.Post(address, "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode > http.StatusOK {
		t.Errorf("bogus HTTP status: %s", resp.Status)
	}

	disclaimers, err := deployment.client.GetDisclaimers(customerID, base.ID(), id.User(base.ID()))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(disclaimers); n != 1 {
		t.Errorf("got %d disclaimers: %#v", n, disclaimers)
	}

	if err := HasAcceptedAllDisclaimers(deployment.client, customerID, base.ID(), id.User(base.ID())); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), fmt.Sprintf("disclaimer=%s is not accepted", disclaimers[0].ID)) {
			t.Errorf("unexpected error: %v", err)
		}
	}

	// Accept the disclaimer and check again
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	if client, ok := deployment.client.(*moovClient); ok {
		_, resp, err = client.underlying.CustomersApi.AcceptDisclaimer(ctx, customerID, disclaimers[0].ID, nil)
		if err != nil {
			t.Error(err)
		}
		resp.Body.Close()

		if err := HasAcceptedAllDisclaimers(deployment.client, customerID, base.ID(), id.User(base.ID())); err != nil {
			t.Error(err)
		}
	} else {
		t.Errorf("deployment client is a %T", deployment.client)
	}

	deployment.close(t) // close only if successful
}

func TestCustomers__hasAcceptedAllDisclaimers(t *testing.T) {
	client := &TestClient{
		Disclaimers: []Disclaimer{
			{
				ID:   base.ID(),
				Text: "requirements",
			},
		},
	}
	customerID := base.ID()

	if err := HasAcceptedAllDisclaimers(client, customerID, base.ID(), id.User(base.ID())); err == nil {
		t.Error("expected error (unaccepted disclaimer)")
	}

	client.Disclaimers[0].AcceptedAt = time.Now()
	if err := HasAcceptedAllDisclaimers(client, customerID, base.ID(), id.User(base.ID())); err != nil {
		t.Errorf("expected no error: %v", err)
	}

	client.Err = errors.New("bad error")
	if err := HasAcceptedAllDisclaimers(client, customerID, base.ID(), id.User(base.ID())); err == nil {
		t.Error("expeced error")
	}
}

func TestCustomers__OFACSearch(t *testing.T) {
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

	_, err = deployment.client.LatestOFACSearch(cust.ID, "requestID", "userID")
	if err != nil {
		t.Fatal(err)
	}

	result, err := deployment.client.RefreshOFACSearch(cust.ID, "requestID", "userID")
	if err != nil {
		t.Fatal(err)
	}
	if result.EntityId == "" || result.Match < 0.01 {
		t.Errorf("result=%#v", result)
	}

	deployment.close(t)
}
