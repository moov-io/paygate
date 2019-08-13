// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/base/docker"
	ofac "github.com/moov-io/ofac/client"

	"github.com/go-kit/kit/log"
	"github.com/ory/dockertest"
)

type testOFACClient struct {
	company  *ofac.OfacCompany
	customer *ofac.OfacCustomer
	sdn      *ofac.Sdn

	// error to be returned instead of field from above
	err error
}

func (c *testOFACClient) Ping() error {
	return c.err
}

func (c *testOFACClient) GetCompany(_ context.Context, id string) (*ofac.OfacCompany, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.company, nil
}

func (c *testOFACClient) GetCustomer(_ context.Context, id string) (*ofac.OfacCustomer, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.customer, nil
}

func (c *testOFACClient) Search(_ context.Context, name string, _ string) (*ofac.Sdn, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.sdn, nil
}

func TestOFAC__matchThreshold(t *testing.T) {
	cases := []struct {
		in string
		v  float32
	}{
		{"", 0.0},
		{"0.25", 0.25},
		{"bad", 0.0},
	}
	for i := range cases {
		v, _ := getOFACMatchThreshold(cases[i].in)
		if math.Abs(float64(v-cases[i].v)) > 0.01 {
			t.Errorf("OFAC_MATCH_THRESHOLD=%s failed, got %.2f", cases[i].in, v)
		}
	}
}

type ofacDeployment struct {
	res    *dockertest.Resource
	client OFACClient
}

func (d *ofacDeployment) close(t *testing.T) {
	if err := d.res.Close(); err != nil {
		t.Error(err)
	}
}

func spawnOFAC(t *testing.T) *ofacDeployment {
	// no t.Helper() call so we know where it failed

	if testing.Short() {
		t.Skip("-short flag enabled")
	}
	if !docker.Enabled() {
		t.Skip("Docker not enabled")
	}

	// Spawn OFAC docker image
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatal(err)
	}
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "moov/ofac",
		Tag:        "v0.10.0-rc3",
		Cmd:        []string{"-http.addr=:8080"},
	})
	if err != nil {
		t.Fatal(err)
	}

	addr := fmt.Sprintf("http://localhost:%s", resource.GetPort("8080/tcp"))
	client := newOFACClient(log.NewNopLogger(), addr, nil)
	err = pool.Retry(func() error {
		return client.Ping()
	})
	if err != nil {
		t.Fatal(err)
	}
	return &ofacDeployment{resource, client}
}

func TestOFAC__client(t *testing.T) {
	endpoint := ""
	if client := newOFACClient(log.NewNopLogger(), endpoint, nil); client == nil {
		t.Fatal("expected non-nil client")
	}

	// Spawn an OFAC Docker image and ping against it
	deployment := spawnOFAC(t)
	if err := deployment.client.Ping(); err != nil {
		t.Fatal(err)
	}
	deployment.close(t) // close only if successful
}

func TestOFAC__get(t *testing.T) {
	ctx := context.TODO()

	deployment := spawnOFAC(t)
	customer, err := deployment.client.GetCustomer(ctx, "22790")
	if customer == nil || err != nil {
		t.Errorf("customer=%v err=%v", customer, err)
	}

	company, err := deployment.client.GetCompany(ctx, "22905")
	if company == nil || err != nil {
		t.Errorf("company=%v err=%v", company, err)
	}

	deployment.close(t) // only if rest of test was successful

	// error cases
	client := newOFACClient(log.NewNopLogger(), "http://localhost:9999", nil)

	customer, err = client.GetCustomer(ctx, "100000")
	if customer != nil || err == nil {
		t.Errorf("expected error: customer=%v err=%v", customer, err)
	}
	company, err = client.GetCompany(ctx, "100000")
	if company != nil || err == nil {
		t.Errorf("expected error: company=%v err=%v", company, err)
	}
}

func TestOFAC__search(t *testing.T) {
	ctx := context.TODO()

	deployment := spawnOFAC(t)

	// Search query that matches an SDN higher than an AltName
	sdn, err := deployment.client.Search(ctx, "Nicolas Maduro", base.ID())
	if err != nil || sdn == nil {
		t.Fatalf("sdn=%v err=%v", sdn, err)
	}
	if sdn.EntityID != "22790" {
		t.Errorf("SDN=%s %#v", sdn.EntityID, sdn)
	}

	// Search query that matches an AltName higher than SDN
	sdn, err = deployment.client.Search(ctx, "Osama bin Muhammad bin Awad BIN LADIN", base.ID())
	if err != nil || sdn == nil {
		t.Fatalf("sdn=%v err=%v", sdn, err)
	}
	if sdn.EntityID != "6365" {
		t.Errorf("SDN=%s %#v", sdn.EntityID, sdn)
	}

	deployment.close(t) // close only if successful
}

func TestOFAC_ping(t *testing.T) {
	client := &testOFACClient{}

	// Ping tests
	if err := client.Ping(); err != nil {
		t.Error("expected no error")
	}

	// set error and verify we get it
	client.err = errors.New("ping error")
	if err := client.Ping(); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "ping error") {
			t.Errorf("unknown error: %v", err)
		}
	}
}

func TestOFAC__rejectViaOFACMatch(t *testing.T) {
	logger := log.NewNopLogger()

	client := &testOFACClient{
		sdn: &ofac.Sdn{}, // non-nil to avoid panic
		err: errors.New("searchOFAC error"),
	}

	if err := rejectViaOFACMatch(logger, client, "name", "userId", ""); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), `ofac: blocking "name" due to OFAC error`) {
			t.Fatalf("unknown error: %v", err)
		}
	}

	// unsafe Customer
	client = &testOFACClient{
		sdn: &ofac.Sdn{
			SdnType: "individual",
		},
		customer: &ofac.OfacCustomer{
			Status: ofac.OfacCustomerStatus{
				Status: "unsafe",
			},
		},
	}
	if err := rejectViaOFACMatch(logger, client, "name", "userId", ""); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "marked unsafe") {
			t.Fatalf("unknown error: %v", err)
		}
	}

	// high match
	client = &testOFACClient{
		sdn: &ofac.Sdn{
			SdnType: "individual",
			Match:   0.99,
		},
		customer: &ofac.OfacCustomer{}, // non-nil to avoid panic
	}
	if err := rejectViaOFACMatch(logger, client, "name", "userId", ""); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "ofac: blocking due to OFAC match=0.99") {
			t.Fatalf("unknown error: %v", err)
		}
	}

	// no results, happy path
	client = &testOFACClient{}
	if err := rejectViaOFACMatch(logger, client, "jane doe", "userId", ""); err != nil {
		t.Fatalf("expected no error, but got %v", err)
	}
}
