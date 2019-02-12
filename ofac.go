// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/base/k8s"
	ofac "github.com/moov-io/ofac/client"

	"github.com/antihax/optional"
	"github.com/go-kit/kit/log"
)

func ofacClient(logger log.Logger) *ofac.APIClient {
	conf := ofac.NewConfiguration()
	conf.BasePath = "http://localhost" + bind.HTTP("ofac")
	if k8s.Inside() {
		conf.BasePath = "http://ofac.apps.svc.cluster.local:8080"
	}

	// OFAC_ENDPOINT is a DNS record responsible for routing us to an ACH instance.
	// Example: http://ofac.apps.svc.cluster.local:8080
	if v := os.Getenv("OFAC_ENDPOINT"); v != "" {
		conf.BasePath = v
	}

	logger.Log("ofac", fmt.Sprintf("using %s for OFAC address", conf.BasePath))

	return ofac.NewAPIClient(conf)
}

// lookupCustomerOFAC will attempt a search for the Customer metadata in OFAC and return a result. A result typically indicates
// a match and thus the Customer needs to be blocked from making transactions.
func lookupCustomerOFAC(api *ofac.APIClient, cust *Customer) (*ofac.OfacCustomer, *ofac.Sdn, error) {
	if cust.Metadata == "" {
		return nil, nil, errors.New("empty Customer.Metadata")
	}

	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	sdn, err := searchSDNs(ctx, api, cust)
	if err != nil {
		return nil, nil, err
	}
	if sdn == nil {
		return nil, nil, nil // nothing found
	}
	ofacCustomer, err := getOFACCustomer(ctx, api, sdn.EntityID)
	return ofacCustomer, sdn, err
}

func searchSDNs(ctx context.Context, api *ofac.APIClient, cust *Customer) (*ofac.Sdn, error) {
	search, resp, err := api.OFACApi.SearchSDNs(ctx, &ofac.SearchSDNsOpts{
		Name: optional.NewString(cust.Metadata),
	})
	if err != nil {
		return nil, fmt.Errorf("lookupCustomerOFAC: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("lookupCustomerOFAC customer=%q (status code: %d): %v", cust.ID, resp.StatusCode, err)
	}
	if len(search.SDNs) == 0 {
		return nil, nil // no OFAC results found, so cust not blocked
	}
	return &search.SDNs[0], nil // return first match (we assume it's the higest match)
}

func getOFACCustomer(ctx context.Context, api *ofac.APIClient, id string) (*ofac.OfacCustomer, error) {
	cust, resp, err := api.OFACApi.GetCustomer(ctx, id, nil)
	if err != nil {
		return nil, fmt.Errorf("lookupCustomerOFAC: GetCustomer=%q: %v", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("lookupCustomerOFAC: GetCustomer=%q (status code: %d): %v", cust.Id, resp.StatusCode, err)
	}
	return &cust, nil
}
