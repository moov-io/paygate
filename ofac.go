// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
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

// searchOFAC will attempt a search for the SDN metadata in OFAC and return a result. Any results are
// returned with their match percent and callers MUST verify to reject or block from making transactions.
//
// The string returned represents the ofac.OfacCustomerStatus or ofac.OfacCompanyStatus. Both strings can
// only be "unsafe" (block) or "exception" (never block). Callers MUST verify the status.
func searchOFAC(api *ofac.APIClient, name string) (*ofac.Sdn, string, error) {
	if name == "" {
		return nil, "", errors.New("empty Customer or Company Metadata")
	}

	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	sdn, err := searchSDNs(ctx, api, name)
	if err != nil {
		return nil, "", err
	}
	if sdn == nil {
		return nil, "", nil // nothing found
	}

	// Get customer or company status
	if strings.EqualFold(sdn.SdnType, "individual") {
		cust, err := getOFACCustomer(ctx, api, sdn.EntityID)
		if err == nil {
			return errIfUnsafe(sdn, cust.Status.Status, cust.Status.CreatedAt)
		}
	} else {
		company, err := getOFACCompany(ctx, api, sdn.EntityID)
		if err == nil {
			return errIfUnsafe(sdn, company.Status.Status, company.Status.CreatedAt)
		}
	}
	return nil, "", fmt.Errorf("searchOFAC=%q error=%q", name, err)
}

func errIfUnsafe(sdn *ofac.Sdn, status string, createdAt time.Time) (*ofac.Sdn, string, error) {
	switch strings.ToLower(status) {
	case "unsafe":
		return sdn, status, fmt.Errorf("SDN=%s marked unsafe - blocked", sdn.EntityID)
	case "exception":
		return sdn, status, nil // never block
	default:
		if status == "" && createdAt.IsZero() {
			return sdn, status, nil // no status override, so let caller check match percent
		}
		return sdn, status, fmt.Errorf("unknown SDN status (%s): %#v", status, sdn)
	}
}

// searchSDNs calls into OFAC's search endpoint for given customer metadata (name)
func searchSDNs(ctx context.Context, api *ofac.APIClient, name string) (*ofac.Sdn, error) {
	search, resp, err := api.OFACApi.SearchSDNs(ctx, &ofac.SearchSDNsOpts{
		Name: optional.NewString(name),
	})
	if err != nil {
		return nil, fmt.Errorf("searchSDNs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("searchSDNs customer=%q (status code: %d): %v", name, resp.StatusCode, err)
	}
	if len(search.SDNs) == 0 {
		return nil, nil // no OFAC results found, so cust not blocked
	}
	return &search.SDNs[0], nil // return first match (we assume it's the highest match)
}

// getOFACCustomer looks up a specific OFAC EntityID for associated data linked to an SDN
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

// getOFACCompany looks up a specific OFAC EntityID for associated data linked to an SDN
func getOFACCompany(ctx context.Context, api *ofac.APIClient, id string) (*ofac.OfacCompany, error) {
	company, resp, err := api.OFACApi.GetCompany(ctx, id, nil)
	if err != nil {
		return nil, fmt.Errorf("lookupCompanyOFAC: GetCompany=%q: %v", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("lookupCompanyOFAC: GetCompany=%q (status code: %d): %v", company.Id, resp.StatusCode, err)
	}
	return &company, nil
}
