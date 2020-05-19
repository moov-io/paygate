// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
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
		Metadata: make(map[string]string),
	}
	msg.Metadata["transferID"] = xfer.Transfer.TransferID

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(xfer); err != nil {
		return fmt.Errorf("trasferID=%s json encode: %v", xfer.Transfer.TransferID, err)
	}
	msg.Body = buf.Bytes()

	return pub.topic.Send(context.TODO(), msg)
}

func (pub *streamPublisher) Cancel(cancel CanceledTransfer) error {
	msg := &pubsub.Message{
		Metadata: make(map[string]string),
	}
	msg.Metadata["transferID"] = cancel.TransferID

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(cancel); err != nil {
		return fmt.Errorf("trasferID=%s json encode: %v", cancel.TransferID, err)
	}
	msg.Body = buf.Bytes()

	return pub.topic.Send(context.TODO(), msg)
}

func (pub *streamPublisher) Shutdown(ctx context.Context) {
	if pub == nil {
		return
	}
	pub.topic.Shutdown(ctx)
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
