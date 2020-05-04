// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/moov-io/paygate/pkg/config"

	"gocloud.dev/pubsub"
)

type streamPublisher struct {
	topic *pubsub.Topic
}

func (pub *streamPublisher) Upload(xfer Xfer) error {
	msg := &pubsub.Message{
		Metadata: createMetadata(xfer),
	}
	if body, err := createBody(xfer); err != nil {
		return err
	} else {
		msg.Body = body
	}

	fmt.Printf("\nstream: upload:\n  body=%v\nn", string(msg.Body))

	return pub.topic.Send(context.TODO(), msg)
}

func (pub *streamPublisher) Cancel(xfer Xfer) error {
	return nil // TODO(adam): impl
}

func createStreamPublisher(cfg *config.StreamPipeline) (XferPublisher, error) {
	if cfg == nil {
		return nil, errors.New("missing config: StreamPipeline")
	}
	if cfg.InMem != nil {
		return inmemPublisher(cfg.InMem.URL)
	}
	if cfg.Kafka != nil {
		return createKafkaPublisher(cfg.Kafka)
	}
	return nil, errors.New("unknown StreamPipeline config")
}
