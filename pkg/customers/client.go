// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/base/k8s"
	moovcustomers "github.com/moov-io/customers/client"

	"github.com/antihax/optional"
	"github.com/go-kit/kit/log"
)

type Client interface {
	Ping() error

	Lookup(customerID string, requestID string, userID string) (*moovcustomers.Customer, error)
	FindAccount(customerID, accountID string) (*moovcustomers.Account, error)
	DecryptAccount(customerID, accountID string) (*moovcustomers.TransitAccountNumber, error)

	LatestOFACSearch(customerID, requestID string, userID string) (*OfacSearch, error)
	RefreshOFACSearch(customerID, requestID string, userID string) (*OfacSearch, error)
}

type moovClient struct {
	underlying *moovcustomers.APIClient
	logger     log.Logger
}

func (c *moovClient) Ping() error {
	// create a context just for this so ping requests don't require the setup of one
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	resp, err := c.underlying.CustomersApi.Ping(ctx)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil || err != nil {
		return fmt.Errorf("customers Ping: failed: %v", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("customers Ping: got status: %s", resp.Status)
	}
	return err
}

func (c *moovClient) Lookup(customerID string, requestID string, userID string) (*moovcustomers.Customer, error) {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	cust, resp, err := c.underlying.CustomersApi.GetCustomer(ctx, customerID, &moovcustomers.GetCustomerOpts{
		XRequestID: optional.NewString(requestID),
		XUserID:    optional.NewString(userID),
	})
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil || err != nil {
		return nil, fmt.Errorf("lookup customer: failed: %v", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("lookup customer: status=%s", resp.Status)
	}
	return &cust, nil
}

func (c *moovClient) FindAccount(customerID, accountID string) (*moovcustomers.Account, error) {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	accounts, resp, err := c.underlying.CustomersApi.GetCustomerAccounts(ctx, customerID, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil || err != nil {
		return nil, fmt.Errorf("lookup customer: failed: %v", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("lookup customer: status=%s", resp.Status)
	}
	for i := range accounts {
		if accounts[i].AccountID == accountID {
			return &accounts[i], nil
		}
	}
	return nil, fmt.Errorf("accountID=%s not found for customerID=%s", accountID, customerID)
}

func (c *moovClient) DecryptAccount(customerID, accountID string) (*moovcustomers.TransitAccountNumber, error) {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	transit, resp, err := c.underlying.CustomersApi.DecryptAccountNumber(ctx, customerID, accountID, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil || err != nil {
		return nil, fmt.Errorf("lookup customer: failed: %v", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("lookup customer: status=%s", resp.Status)
	}
	return &transit, nil
}

func (c *moovClient) LatestOFACSearch(customerID, requestID string, userID string) (*OfacSearch, error) {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	result, resp, err := c.underlying.CustomersApi.GetLatestOFACSearch(ctx, customerID, &moovcustomers.GetLatestOFACSearchOpts{
		XRequestID: optional.NewString(requestID),
		XUserID:    optional.NewString(userID),
	})
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil || err != nil {
		return nil, fmt.Errorf("get latest OFAC search: %v", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("get latest OFAC search: status=%s", resp.Status)
	}
	return &OfacSearch{
		EntityId:  result.EntityID,
		SdnName:   result.SdnName,
		Match:     result.Match,
		CreatedAt: result.CreatedAt,
	}, nil
}

func (c *moovClient) RefreshOFACSearch(customerID, requestID string, userID string) (*OfacSearch, error) {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	result, resp, err := c.underlying.CustomersApi.RefreshOFACSearch(ctx, customerID, &moovcustomers.RefreshOFACSearchOpts{
		XRequestID: optional.NewString(requestID),
		XUserID:    optional.NewString(userID),
	})
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil || err != nil {
		return nil, fmt.Errorf("refresh OFAC search: %v", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("refresh OFAC search: status=%s", resp.Status)
	}
	return &OfacSearch{
		EntityId:  result.EntityID,
		SdnName:   result.SdnName,
		Match:     result.Match,
		CreatedAt: result.CreatedAt,
	}, nil
}

// NewClient returns an Client instance and will default to using the Customers address in
// moov's standard Kubernetes setup.
//
// endpoint is a DNS record responsible for routing us to an Customers instance.
// Example: http://customers.apps.svc.cluster.local:8080
func NewClient(logger log.Logger, endpoint string, httpClient *http.Client) Client {
	conf := moovcustomers.NewConfiguration()
	conf.BasePath = "http://localhost" + bind.HTTP("customers")
	conf.HTTPClient = httpClient

	if k8s.Inside() {
		conf.BasePath = "http://customers.apps.svc.cluster.local:8080"
	}
	if endpoint != "" {
		conf.BasePath = endpoint // override from provided CUSTOMERS_ENDPOINT env variable
	}

	logger.Log("customers", fmt.Sprintf("using %s for Customers address", conf.BasePath))

	return &moovClient{
		underlying: moovcustomers.NewAPIClient(conf),
		logger:     logger,
	}
}
