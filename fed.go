// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package paygate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/base/k8s"
	fed "github.com/moov-io/fed/client"

	"github.com/antihax/optional"
	"github.com/go-kit/kit/log"
)

type FEDClient interface {
	Ping() error
	LookupRoutingNumber(routingNumber string) error
}

type moovFEDClient struct {
	underlying *fed.APIClient
	logger     log.Logger
}

func (c *moovFEDClient) Ping() error {
	// create a context just for this so ping requests don't require the setup of one
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	resp, err := c.underlying.FEDApi.Ping(ctx)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil {
		return fmt.Errorf("FED ping failed: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("FED ping got status: %s", resp.Status)
	}
	return err
}

func (c *moovFEDClient) LookupRoutingNumber(routingNumber string) error {
	// create a context just for this so ping requests don't require the setup of one
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	achDict, resp, err := c.underlying.FEDApi.SearchFEDACH(ctx, &fed.SearchFEDACHOpts{
		RoutingNumber: optional.NewString(routingNumber),
	})
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil {
		return fmt.Errorf("FED ping failed: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("FED ping got status: %s", resp.Status)
	}
	for i := range achDict.ACHParticipants {
		if achDict.ACHParticipants[i].RoutingNumber == routingNumber {
			return nil // found match
		}
	}
	return errors.New("no ACH participants found")
}

func CreateFEDClient(logger log.Logger, httpClient *http.Client) FEDClient {
	conf := fed.NewConfiguration()
	conf.BasePath = "http://localhost" + bind.HTTP("fed")
	conf.HTTPClient = httpClient

	if k8s.Inside() {
		conf.BasePath = "http://fed.apps.svc.cluster.local:8080"
	}

	// FED_ENDPOINT is a DNS record responsible for routing us to an FED instance.
	// Example: http://fed.apps.svc.cluster.local:8080
	if v := os.Getenv("FED_ENDPOINT"); v != "" {
		conf.BasePath = v
	}

	logger.Log("fed", fmt.Sprintf("using %s for FED address", conf.BasePath))

	return &moovFEDClient{
		underlying: fed.NewAPIClient(conf),
		logger:     logger,
	}
}
