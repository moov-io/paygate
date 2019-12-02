// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/antihax/optional"
	"github.com/go-kit/kit/log"

	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/base/k8s"
	moovcustomers "github.com/moov-io/customers/client"
	"github.com/moov-io/paygate/pkg/id"
)

type Client interface {
	Ping() error

	Create(opts *Request) (*moovcustomers.Customer, error)
	Lookup(customerID string, requestID string, userID id.User) (*moovcustomers.Customer, error)

	GetDisclaimers(customerID, requestID string, userID id.User) ([]moovcustomers.Disclaimer, error)

	LatestOFACSearch(customerID, requestID string, userID id.User) (*moovcustomers.OfacSearch, error)
	RefreshOFACSearch(customerID, requestID string, userID id.User) (*moovcustomers.OfacSearch, error)
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

type Request struct {
	Name      string
	Email     string
	SSN       string
	BirthDate time.Time

	Phones    []moovcustomers.CreatePhone
	Addresses []moovcustomers.CreateAddress

	RequestID string
	UserID    id.User
}

func breakupName(in string) (string, string) {
	parts := strings.Fields(in)
	if len(parts) < 2 {
		return in, ""
	}
	return parts[0], parts[len(parts)-1]
}

func (c *moovClient) Create(opts *Request) (*moovcustomers.Customer, error) {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	first, last := breakupName(opts.Name)
	req := moovcustomers.CreateCustomer{
		FirstName: first,
		LastName:  last,
		BirthDate: opts.BirthDate,
		Phones:    opts.Phones,
		Addresses: opts.Addresses,
		SSN:       opts.SSN,
		Email:     opts.Email,
	}

	cust, resp, err := c.underlying.CustomersApi.CreateCustomer(ctx, req, &moovcustomers.CreateCustomerOpts{
		XRequestID: optional.NewString(opts.RequestID),
		XUserID:    optional.NewString(opts.UserID.String()),
	})
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil || err != nil {
		return nil, fmt.Errorf("customer create: failed: %v", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("customer create: got status: %s", resp.Status)
	}
	return &cust, nil
}

func (c *moovClient) Lookup(customerID string, requestID string, userID id.User) (*moovcustomers.Customer, error) {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	cust, resp, err := c.underlying.CustomersApi.GetCustomer(ctx, customerID, &moovcustomers.GetCustomerOpts{
		XRequestID: optional.NewString(requestID),
		XUserID:    optional.NewString(userID.String()),
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

func (c *moovClient) GetDisclaimers(customerID, requestID string, userID id.User) ([]moovcustomers.Disclaimer, error) {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	disclaimers, resp, err := c.underlying.CustomersApi.GetCustomerDisclaimers(ctx, customerID, &moovcustomers.GetCustomerDisclaimersOpts{
		XRequestID: optional.NewString(requestID),
		XUserID:    optional.NewString(userID.String()),
	})
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil || err != nil {
		return nil, fmt.Errorf("get customer disclaimers: failed: %v", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("get customer disclaimers: status=%s", resp.Status)
	}
	return disclaimers, nil
}

func (c *moovClient) LatestOFACSearch(customerID, requestID string, userID id.User) (*moovcustomers.OfacSearch, error) {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	result, resp, err := c.underlying.CustomersApi.GetLatestOFACSearch(ctx, customerID, &moovcustomers.GetLatestOFACSearchOpts{
		XRequestID: optional.NewString(requestID),
		XUserID:    optional.NewString(userID.String()),
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
	return &result, nil
}

func (c *moovClient) RefreshOFACSearch(customerID, requestID string, userID id.User) (*moovcustomers.OfacSearch, error) {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	result, resp, err := c.underlying.CustomersApi.RefreshOFACSearch(ctx, customerID, &moovcustomers.RefreshOFACSearchOpts{
		XRequestID: optional.NewString(requestID),
		XUserID:    optional.NewString(userID.String()),
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
	return &result, nil
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

// HasAcceptedAllDisclaimers will return an error if there's a disclaimer which has not been accepted
// for the given customerID. If no disclaimers exist or all have been accepted a nil error will be returned.
func HasAcceptedAllDisclaimers(client Client, customerID string, requestID string, userID id.User) error {
	ds, err := client.GetDisclaimers(customerID, requestID, userID)
	if err != nil {
		return err
	}
	for i := range ds {
		// The Customers service claims that any disclaimer accepted before the year 2000 isn't actually
		// accepted, so we mirror logic.
		if t := ds[i].AcceptedAt; t.IsZero() || t.Year() < 2000 {
			return fmt.Errorf("disclaimer=%s is not accepted", ds[i].ID)
		}
	}
	return nil
}
