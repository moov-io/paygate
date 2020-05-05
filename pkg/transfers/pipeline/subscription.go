// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/stream"
	"gocloud.dev/pubsub"
)

func NewSubscription(cfg *config.Config) (*pubsub.Subscription, error) {
	if cfg == nil {
		return nil, errors.New("nil Config")
	}
	if cfg.Pipeline.Stream != nil {
		return createStreamSubscription(cfg.Pipeline.Stream)
	}
	return nil, errors.New("unknown Pipeline config")
}

func createStreamSubscription(cfg *config.StreamPipeline) (*pubsub.Subscription, error) {
	if cfg.InMem != nil {
		return createInmemSubscription(cfg.InMem.URL)
	}
	return nil, fmt.Errorf("unknown %#v", cfg) // TODO(adam):
}

func createInmemSubscription(url string) (*pubsub.Subscription, error) {
	return stream.Subscription(context.TODO(), url)
}
