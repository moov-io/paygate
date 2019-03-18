// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/base/k8s"
	gl "github.com/moov-io/gl/client"

	// "github.com/antihax/optional"
	"github.com/go-kit/kit/log"
)

type GLClient interface {
	Ping() error

	GetAccounts(customerId string) ([]gl.Account, error)
}

type moovGLClient struct {
	underlying *gl.APIClient
	logger     log.Logger
}

func (c *moovGLClient) Ping() error {
	// create a context just for this so ping requests don't require the setup of one
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	resp, err := c.underlying.GLApi.Ping(ctx)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil {
		return fmt.Errorf("GL ping failed: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("GL ping got status: %s", resp.Status)
	}
	return err
}

func (c *moovGLClient) GetAccounts(customerId string) ([]gl.Account, error) {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	accounts, resp, err := c.underlying.GLApi.GetAccountsByCustomerID(ctx, customerId)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil {
		return nil, fmt.Errorf("GL GetAccounts(%q) failed: %v", customerId, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("GL GetAccounts(%q) bogus HTTP status: %s", customerId, resp.Status)
	}
	return accounts, nil
}

func createGLClient(logger log.Logger) GLClient {
	conf := gl.NewConfiguration()
	conf.BasePath = "http://localhost" + bind.HTTP("gl")

	if k8s.Inside() {
		conf.BasePath = "http://gl.apps.svc.cluster.local:8080"
	}

	// GL_ENDPOINT is a DNS record responsible for routing us to an GL instance.
	// Example: http://gl.apps.svc.cluster.local:8080
	if v := os.Getenv("GL_ENDPOINT"); v != "" {
		conf.BasePath = v
	}

	logger.Log("gl", fmt.Sprintf("using %s for GL address", conf.BasePath))

	return &moovGLClient{
		underlying: gl.NewAPIClient(conf),
		logger:     logger,
	}
}
