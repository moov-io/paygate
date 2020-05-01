// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package stream

import (
	"context"
	"testing"

	"gocloud.dev/pubsub"
)

func TestStream(t *testing.T) {
	topicURL := "mem://moov"
	ctx := context.Background()

	topic, err := Topic(ctx, topicURL)
	if err != nil {
		t.Fatal(err)
	}
	defer topic.Shutdown(ctx)

	sub, err := Subscription(ctx, topicURL)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Shutdown(ctx)

	// quick send and receive
	send(ctx, topic, "hello, world")
	if msg, err := receive(ctx, sub); err == nil {
		if msg != "hello, world" {
			t.Errorf("got %q", msg)
		}
	} else {
		t.Fatal(err)
	}
}

func send(ctx context.Context, t *pubsub.Topic, body string) *pubsub.Message {
	msg := &pubsub.Message{
		Body:     []byte(body),
		Metadata: make(map[string]string),
	}
	t.Send(ctx, msg)
	return msg
}

func receive(ctx context.Context, t *pubsub.Subscription) (string, error) {
	msg, err := t.Receive(ctx)
	if err != nil {
		return "", err
	}
	return string(msg.Body), nil
}
