// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	ofac "github.com/moov-io/ofac/client"

	"github.com/go-kit/kit/log"
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

func (c *testOFACClient) Search(_ context.Context, name string) (*ofac.Sdn, error) {
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

func TestOFAC__client(t *testing.T) {
	if client := ofacClient(log.NewNopLogger()); client == nil {
		t.Fatal("expected non-nil client")
	}
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

	if err := rejectViaOFACMatch(logger, client, "name", "userId"); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), `ofac: blocking "name" due to OFAC match`) {
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
	if err := rejectViaOFACMatch(logger, client, "name", "userId"); err == nil {
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
	if err := rejectViaOFACMatch(logger, client, "name", "userId"); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "ofac: blocking due to OFAC match=0.99") {
			t.Fatalf("unknown error: %v", err)
		}
	}

	// no results, happy path
	client = &testOFACClient{}
	if err := rejectViaOFACMatch(logger, client, "jane doe", "userId"); err != nil {
		t.Fatalf("expected no error, but got %v", err)
	}
}
