// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/base/k8s"
	moovcustomers "github.com/moov-io/customers/client"
)

type Client interface {
	Ping() error
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
	if resp == nil {
		return fmt.Errorf("customers Ping: failed: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("customers Ping: got status: %s", resp.Status)
	}
	return err
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
