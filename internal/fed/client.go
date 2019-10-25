// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package fed

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/base/k8s"
	moovfed "github.com/moov-io/fed/client"

	"github.com/antihax/optional"
	"github.com/go-kit/kit/log"
)

type Client interface {
	Ping() error
	LookupRoutingNumber(routingNumber string) error
}

type moovClient struct {
	underlying *moovfed.APIClient
	logger     log.Logger
}

func (c *moovClient) Ping() error {
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

func (c *moovClient) LookupRoutingNumber(routingNumber string) error {
	// create a context just for this so ping requests don't require the setup of one
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	achDict, resp, err := c.underlying.FEDApi.SearchFEDACH(ctx, &moovfed.SearchFEDACHOpts{
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

func NewClient(logger log.Logger, endpoint string, httpClient *http.Client) Client {
	conf := moovfed.NewConfiguration()
	conf.HTTPClient = httpClient

	if endpoint != "" {
		conf.BasePath = endpoint
	} else {
		if k8s.Inside() {
			conf.BasePath = "http://fed.apps.svc.cluster.local:8080"
		} else {
			conf.BasePath = "http://localhost" + bind.HTTP("fed")
		}
	}
	logger.Log("fed", fmt.Sprintf("using %s for FED address", conf.BasePath))

	return &moovClient{
		underlying: moovfed.NewAPIClient(conf),
		logger:     logger,
	}
}
