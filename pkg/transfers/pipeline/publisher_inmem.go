// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moov-io/paygate/pkg/stream"

	"gocloud.dev/pubsub"
	"gocloud.dev/pubsub/mempubsub"
)

func inmemPublisher(url string) (*streamPublisher, error) {
	topic, err := stream.Topic(context.TODO(), url)
	if err != nil {
		return nil, err
	}
	return &streamPublisher{topic: topic}, nil
}

func testingPublisher(t *testing.T) *streamPublisher {
	pub, err := inmemPublisher(fmt.Sprintf("mem://%s", t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pub.Shutdown(context.Background()) })
	return pub
}

func testingSubscriber(t *testing.T, pub *streamPublisher) *pubsub.Subscription {
	return mempubsub.NewSubscription(pub.topic, 1*time.Second)
}
