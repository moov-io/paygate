// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"context"
	"fmt"

	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/events"
	"github.com/moov-io/paygate/pkg/stream"

	"github.com/go-kit/kit/log"
	"gocloud.dev/pubsub"
)

type streamEmitter struct {
	logger      log.Logger
	adminConfig config.Admin

	ctx   context.Context
	topic *pubsub.Topic
}

func NewStreamEmitter(cfg *config.Config) (*streamEmitter, error) {
	if cfg.Transfers.Inbound.Stream == nil {
		return nil, nil
	}

	ctx := context.Background()
	topic, err := stream.OpenTopic(ctx, cfg.Transfers.Inbound.Stream)
	if err != nil {
		return nil, fmt.Errorf("inbound: %v", err)
	}
	return &streamEmitter{
		adminConfig: cfg.Admin,
		logger:      cfg.Logger,
		ctx:         ctx,
		topic:       topic,
	}, nil
}

func (pc *streamEmitter) Type() string {
	return "stream"
}

func (pc *streamEmitter) Handle(event File) error {
	switch {
	case isCorrectionFile(event.File):
		pc.sendEvent(events.CreateCorrectionFileEvent(pc.adminConfig, event.Filename, event.File))

	case isPrenoteFile(event.File):
		pc.sendEvent(events.CreatePrenoteFileEvent(pc.adminConfig, event.Filename, event.File))

	case isReturnFile(event.File):
		pc.sendEvent(events.CreateReturnFileEvent(pc.adminConfig, event.Filename, event.File))
	}
	return fmt.Errorf("unhandled inbound File: %s", event.Filename)
}

func (pc *streamEmitter) sendEvent(msg *pubsub.Message, err error) {
	if msg == nil || err != nil {
		pc.logger.Log("inbound", fmt.Sprintf("stream: %v", err))
		return
	}
	if err := pc.topic.Send(pc.ctx, msg); err != nil {
		pc.logger.Log("inbound", fmt.Sprintf("stream: problem sending message: %v", err))
	}
}
