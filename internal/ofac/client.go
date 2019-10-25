// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package ofac

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/base/k8s"
	moovofac "github.com/moov-io/ofac/client"

	"github.com/antihax/optional"
	"github.com/go-kit/kit/log"
)

var (
	OFACMatchThreshold float32 = 0.99
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

type Client interface {
	Ping() error

	GetCompany(ctx context.Context, id string) (*moovofac.OfacCompany, error)
	GetCustomer(ctx context.Context, id string) (*moovofac.OfacCustomer, error)

	Search(ctx context.Context, name string, requestID string) (*moovofac.Sdn, error)
}

type moovClient struct {
	underlying *moovofac.APIClient
	logger     log.Logger
}

func (c *moovClient) Ping() error {
	// create a context just for this so ping requests don't require the setup of one
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	resp, err := c.underlying.OFACApi.Ping(ctx)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil {
		return fmt.Errorf("ofac.Ping: failed: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("ofac.Ping: got status: %s", resp.Status)
	}
	return err
}

func (c *moovClient) GetCompany(ctx context.Context, id string) (*moovofac.OfacCompany, error) {
	company, resp, err := c.underlying.OFACApi.GetOFACCompany(ctx, id, nil)
	if err != nil {
		return nil, fmt.Errorf("ofac.GetCompany: GetCompany=%q: %v", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("ofac.GetCompany: GetCompany=%q (status code: %d): %v", company.ID, resp.StatusCode, err)
	}
	return &company, nil
}

func (c *moovClient) GetCustomer(ctx context.Context, id string) (*moovofac.OfacCustomer, error) {
	cust, resp, err := c.underlying.OFACApi.GetOFACCustomer(ctx, id, nil)
	if err != nil {
		return nil, fmt.Errorf("ofac.GetCustomer: GetCustomer=%q: %v", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("ofac.GetCustomer: GetCustomer=%q (status code: %d): %v", cust.ID, resp.StatusCode, err)
	}
	return &cust, nil
}

// Search returns the top OFAC match given the provided options across SDN names and AltNames
func (c *moovClient) Search(ctx context.Context, name string, requestID string) (*moovofac.Sdn, error) {
	search, resp, err := c.underlying.OFACApi.Search(ctx, &moovofac.SearchOpts{
		Q:          optional.NewString(name),
		Limit:      optional.NewInt32(1),
		XRequestID: optional.NewString(requestID),
	})
	if err != nil {
		return nil, fmt.Errorf("ofac.Search: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("ofac.Search: customer=%q (status code: %d): %v", name, resp.StatusCode, err)
	}
	// We prefer to return the SDN, but if there's an AltName with a higher match return that instead.
	if (len(search.SDNs) > 0 && len(search.AltNames) > 0) && ((search.AltNames[0].Match > 0.1) && (search.AltNames[0].Match > search.SDNs[0].Match)) {
		alt := search.AltNames[0]

		// AltName matched higher than SDN names, so return the SDN of the matched AltName
		sdn, resp, err := c.underlying.OFACApi.GetSDN(ctx, alt.EntityID, &moovofac.GetSDNOpts{
			XRequestID: optional.NewString(requestID),
		})
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("ofac.Search: found alt name: %v", err)
		}
		sdn.Match = alt.Match // copy match from original search (GetSDN doesn't do string matching)
		c.logger.Log("ofac", fmt.Sprintf("AltName=%s,SDN=%s had higher match than SDN=%s", alt.AlternateID, alt.EntityID, search.SDNs[0].EntityID), "requestID", requestID)
		return &sdn, nil
	} else {
		if len(search.SDNs) > 0 {
			return &search.SDNs[0], nil // return the SDN which had a higher match than any AltNames
		}
	}
	return nil, nil // no OFAC results found, so cust not blocked
}

// NewClient returns an Client instance and will default to using the OFAC address in
// moov's standard Kubernetes setup.
//
// endpoint is a DNS record responsible for routing us to an OFAC instance.
// Example: http://ofac.apps.svc.cluster.local:8080
func NewClient(logger log.Logger, endpoint string, httpClient *http.Client) Client {
	conf := moovofac.NewConfiguration()
	conf.HTTPClient = httpClient

	if endpoint != "" {
		conf.BasePath = endpoint
	} else {
		if k8s.Inside() {
			conf.BasePath = "http://ofac.apps.svc.cluster.local:8080"
		} else {
			conf.BasePath = "http://localhost" + bind.HTTP("ofac")
		}
	}
	logger.Log("ofac", fmt.Sprintf("using %s for OFAC address", conf.BasePath))

	return &moovClient{
		underlying: moovofac.NewAPIClient(conf),
		logger:     logger,
	}
}

// RejectViaMatch shares logic for handling the response from searchOFAC
func RejectViaMatch(logger log.Logger, api Client, name string, userId string, requestID string) error {
	sdn, status, err := searchOFAC(api, name, requestID)
	if err != nil {
		if sdn == nil {
			return fmt.Errorf("ofac: blocking %q due to OFAC error: %v", name, err)
		}
		return fmt.Errorf("ofac: blocking SDN=%s due to OFAC error: %v", sdn.EntityID, err)
	}
	if strings.EqualFold(status, "unsafe") {
		return fmt.Errorf("ofac: blocking due to OFAC status=%s SDN=%#v", status, sdn)
	}
	if sdn != nil && sdn.Match > OFACMatchThreshold {
		return fmt.Errorf("ofac: blocking due to OFAC match=%.2f EntityID=%s", sdn.Match, sdn.EntityID)
	}

	if logger != nil {
		if sdn == nil {
			logger.Log("customers", fmt.Sprintf("ofac: no results found for %s", name), "userId", userId, "requestID", requestID)
		} else {
			logger.Log("customers", fmt.Sprintf("ofac: found SDN %s with match %.2f (%s)", sdn.EntityID, sdn.Match, name), "userId", userId, "requestID", requestID)
		}
	}
	return nil
}

// searchOFAC will attempt a search for the SDN metadata in OFAC and return a result. Any results are
// returned with their match percent and callers MUST verify to reject or block from making transactions.
//
// The string returned represents the moovofac.OfacCustomerStatus or moovofac.OfacCompanyStatus. Both strings can
// only be "unsafe" (block) or "exception" (never block). Callers MUST verify the status.
func searchOFAC(api Client, name string, requestID string) (*moovofac.Sdn, string, error) {
	if name == "" {
		return nil, "", errors.New("empty Customer or Company Metadata")
	}
	if requestID == "" {
		requestID = base.ID()
	}

	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	sdn, err := api.Search(ctx, name, requestID)
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

func errIfUnsafe(sdn *moovofac.Sdn, status string, createdAt time.Time) (*moovofac.Sdn, string, error) {
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
