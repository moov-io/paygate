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
	gl "github.com/moov-io/gl/client"

	"github.com/antihax/optional"
	"github.com/go-kit/kit/log"
)

type GLClient interface {
	Ping() error

	PostTransaction(requestId, userId string, lines []transactionLine) (*gl.Transaction, error)
	SearchAccounts(requestId, userId string, dep *Depository) (*gl.Account, error)
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

type transactionLine struct {
	AccountId string
	Purpose   string
	Amount    int32
}

func (c *moovGLClient) PostTransaction(requestId, userId string, lines []transactionLine) (*gl.Transaction, error) {
	if len(lines) == 0 {
		return nil, errors.New("GL: no transactionLine's")
	}

	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	var glLines []gl.TransactionLine
	for i := range lines {
		glLines = append(glLines, gl.TransactionLine{
			AccountId: lines[i].AccountId,
			Purpose:   lines[i].Purpose,
			Amount:    float32(lines[i].Amount),
		})
	}
	req := gl.CreateTransaction{glLines}
	tx, resp, err := c.underlying.GLApi.CreateTransaction(ctx, userId, req, &gl.CreateTransactionOpts{
		XRequestId: optional.NewString(requestId),
	})
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return &tx, fmt.Errorf("GL: PostTransaction: %v", err)
	}
	return &tx, nil
}

func (c *moovGLClient) SearchAccounts(requestId, userId string, dep *Depository) (*gl.Account, error) {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	c.logger.Log("gl", fmt.Sprintf("searching for depository=%s account", dep.ID), "requestId", requestId)

	account, resp, err := c.underlying.GLApi.SearchAccounts(ctx, dep.AccountNumber, dep.RoutingNumber, string(dep.Type), userId, &gl.SearchAccountsOpts{
		XRequestId: optional.NewString(requestId),
	})
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("GL: SearchAccounts: depository=%s userId=%s", dep.ID, userId)
	}
	return &account, nil
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
