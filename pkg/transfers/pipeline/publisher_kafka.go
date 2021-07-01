// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"errors"

	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/stream"

	"gocloud.dev/pubsub/kafkapubsub"
)

func createKafkaPublisher(cfg *config.KafkaPipeline) (*streamPublisher, error) {
	if cfg == nil {
		return nil, errors.New("nil Kafka config")
	}

	pub := &streamPublisher{}
	var err error

	// kafkapubsub.MinimalConfig returns a minimal sarama.Config required for kafkapubsub
	config := kafkapubsub.MinimalConfig()

	pub.topic, err = stream.KafkaTopic(cfg.Brokers, config, cfg.Topic, nil)

	return pub, err
}
