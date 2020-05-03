// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/stream"

	"github.com/Shopify/sarama"
)

func createKafkaPublisher(cfg *config.KafkaPipeline) (*streamPublisher, error) {
	pub := &streamPublisher{}
	var err error

	config := sarama.NewConfig()
	pub.topic, err = stream.KafkaTopic(cfg.Brokers, config, cfg.Topic, nil)

	return pub, err
}
