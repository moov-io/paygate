// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/base/k8s"
	ofac "github.com/moov-io/ofac/client"

	"github.com/antihax/optional"
	"github.com/go-kit/kit/log"
)

var (
	OFACMatchThreshold float32 = 0.90
)

func init() {
	f, err := getOFACMatchThreshold(os.Getenv("OFAC_MATCH_THRESHOLD"))
	if err == nil && f > 0.00 {
		OFACMatchThreshold = f
	}
}

func getOFACMatchThreshold(v string) (float32, error) {
	f, err := strconv.ParseFloat(v, 32)
	return float32(f), err
}

type OFACClient interface {
	Ping() error

	GetCompany(ctx context.Context, id string) (*ofac.OfacCompany, error)
	GetCustomer(ctx context.Context, id string) (*ofac.OfacCustomer, error)

	Search(ctx context.Context, name string) (*ofac.Sdn, error)
}

type moovOFACClient struct {
	underlying *ofac.APIClient
	logger     log.Logger
}

func (c *moovOFACClient) Ping() error {
	// create a context just for this so ping requests don't require the setup of one
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	resp, err := c.underlying.OFACApi.Ping(ctx)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil {
		return fmt.Errorf("OFAC ping failed: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("OFAC ping got status: %s", resp.Status)
	}
	return err
}

func (c *moovOFACClient) GetCompany(ctx context.Context, id string) (*ofac.OfacCompany, error) {
	company, resp, err := c.underlying.OFACApi.GetCompany(ctx, id, nil)
	if err != nil {
		return nil, fmt.Errorf("OFAC.GetCompany: GetCompany=%q: %v", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("OFAC.GetCompany: GetCompany=%q (status code: %d): %v", company.Id, resp.StatusCode, err)
	}
	return &company, nil
}

func (c *moovOFACClient) GetCustomer(ctx context.Context, id string) (*ofac.OfacCustomer, error) {
	cust, resp, err := c.underlying.OFACApi.GetCustomer(ctx, id, nil)
	if err != nil {
		return nil, fmt.Errorf("lookupCustomerOFAC: GetCustomer=%q: %v", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("lookupCustomerOFAC: GetCustomer=%q (status code: %d): %v", cust.Id, resp.StatusCode, err)
	}
	return &cust, nil
}

// Search returns the top OFAC match given the provided options
func (c *moovOFACClient) Search(ctx context.Context, name string) (*ofac.Sdn, error) {
	search, resp, err := c.underlying.OFACApi.SearchSDNs(ctx, &ofac.SearchSDNsOpts{
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

func ofacClient(logger log.Logger) OFACClient {
	conf := ofac.NewConfiguration()
	conf.BasePath = "http://localhost" + bind.HTTP("ofac")
	if k8s.Inside() {
		conf.BasePath = "http://ofac.apps.svc.cluster.local:8080"
	}

	// OFAC_ENDPOINT is a DNS record responsible for routing us to an OFAC instance.
	// Example: http://ofac.apps.svc.cluster.local:8080
	if v := os.Getenv("OFAC_ENDPOINT"); v != "" {
		conf.BasePath = v
	}

	logger.Log("ofac", fmt.Sprintf("using %s for OFAC address", conf.BasePath))

	return &moovOFACClient{
		underlying: ofac.NewAPIClient(conf),
		logger:     logger,
	}
}

// rejectViaOFACMatch shares logic for handling the response from searchOFAC
func rejectViaOFACMatch(logger log.Logger, api OFACClient, name string, userId string) error {
	sdn, status, err := searchOFAC(api, name)
	if err != nil {
		if sdn == nil {
			return fmt.Errorf("ofac: blocking %q due to OFAC match: %v", name, err)
		}
		return fmt.Errorf("ofac: blocking SDN=%s due to OFAC match: %v", sdn.EntityID, err)
	}
	if strings.EqualFold(status, "unsafe") {
		return fmt.Errorf("ofac: blocking due to OFAC status=%s SDN=%#v", status, sdn)
	}
	if sdn != nil && sdn.Match > OFACMatchThreshold {
		return fmt.Errorf("ofac: blocking due to OFAC match=%.2f EntityID=%s", sdn.Match, sdn.EntityID)
	}

	if sdn == nil {
		logger.Log("customers", fmt.Sprintf("ofac: no results found for %s", name), "userId", userId)
	} else {
		logger.Log("customers", fmt.Sprintf("ofac: found SDN %s with match %.2f (%s)", sdn.EntityID, sdn.Match, name), "userId", userId)
	}
	return nil
}

// searchOFAC will attempt a search for the SDN metadata in OFAC and return a result. Any results are
// returned with their match percent and callers MUST verify to reject or block from making transactions.
//
// The string returned represents the ofac.OfacCustomerStatus or ofac.OfacCompanyStatus. Both strings can
// only be "unsafe" (block) or "exception" (never block). Callers MUST verify the status.
func searchOFAC(api OFACClient, name string) (*ofac.Sdn, string, error) {
	if name == "" {
		return nil, "", errors.New("empty Customer or Company Metadata")
	}

	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	sdn, err := api.Search(ctx, name)
	if err != nil {
		return nil, "", err
	}
	if sdn == nil {
		return nil, "", nil // nothing found
	}

	// Get customer or company status
	if strings.EqualFold(sdn.SdnType, "individual") {
		cust, err := api.GetCustomer(ctx, sdn.EntityID)
		if err == nil {
			return errIfUnsafe(sdn, cust.Status.Status, cust.Status.CreatedAt)
		}
	} else {
		company, err := api.GetCompany(ctx, sdn.EntityID)
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
