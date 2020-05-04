// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/upload"

	"github.com/go-kit/kit/log"
	"gocloud.dev/pubsub"
)

// XferAggregator ...
//
// this has a for loop which is triggered on cutoff warning
//  e.g. 10mins before 30mins before cutoff (10 mins is Moov's window, 30mins is ODFI)
// consume as many transfers as possible, then upload.
type XferAggregator struct {
	logger log.Logger

	agent upload.Agent

	merger       XferMerging
	subscription *pubsub.Subscription
}

func NewAggregator(logger log.Logger, agent upload.Agent, merger XferMerging, sub *pubsub.Subscription) *XferAggregator {
	return &XferAggregator{
		logger:       logger,
		agent:        agent,
		merger:       merger,
		subscription: sub,
	}
}

// receive each message of *pubsub.Subscription, detect message type
//   - if Xfer, write into ./mergable/
//   - if Xfer, write/rename as ./mergable/foo.ach.deleted ?
//   - on cutoff merge files

func (xfagg *XferAggregator) Start(ctx context.Context) error {
	// when <-ctx.Done() fires shutdown xfagg.consumer and upload any files we have

	cutoff := time.NewTicker(10 * time.Second)

	for {
		select {
		case <-cutoff.C: // TODO(adam): merge files and upload
			if err := xfagg.merger.WithEachMerged(xfagg.uploadFile); err != nil {
				// TODO(adam): log or something
				xfagg.logger.Log("aggregate", fmt.Sprintf("ERROR inside WithEachMerged: %v", err))
			}

		case msg := <-xfagg.await():
			if err := handleMessage(xfagg.merger, msg); err != nil {
				// TODO(adam): log or something
				xfagg.logger.Log("aggregate", fmt.Sprintf("ERROR handling message: %v", err))
			}

		case <-ctx.Done():
			// TODO(adam): shutdown?
		}
	}

	return nil
}

func (xfagg *XferAggregator) uploadFile(f *ach.File) error {
	// TODO(adam): some sort of filename_template.go interaction here

	fmt.Printf("uploading %v\n", f)

	return nil

	// return xfagg.agent.UploadFile(upload.File{
	// 	// Filename: "",
	// 	// Contents: io.ReadCloser,
	// })
}

func (xfagg *XferAggregator) await() chan *pubsub.Message {
	out := make(chan *pubsub.Message, 1)
	go func() {
		msg, err := xfagg.subscription.Receive(context.Background())
		if err != nil {
			// xfagg.logger.Log("", "")
			// TODO(adam): log, or something
		}
		// TODO(adam): we need to wire through a cancel func
		out <- msg
	}()
	return out
}

// handleMessage attempts to parse a pubsub.Message into a strongly typed message
// which an XferMerging instance can handle.
func handleMessage(merger XferMerging, msg *pubsub.Message) error {
	fmt.Printf("(preview) msg.body=%s\n", string(msg.Body)[:50])

	var xfer Xfer
	if err := json.NewDecoder(bytes.NewReader(msg.Body)).Decode(&xfer); err != nil {
		msg.Nack()
		transferID := msg.Metadata["transferID"]
		return fmt.Errorf("problem decoding for transferID=%s: %v", transferID, err)
	}
	fmt.Printf("parsed Xfer=%v\n", xfer)
	if err := merger.HandleXfer(xfer); err != nil {
		msg.Nack()
		return fmt.Errorf("HandleXfer problem with transferID=%s: %v", xfer.Transfer.TransferID, err)
	}

	msg.Ack()

	// TODO(adam): handle Cancel / HandleCancel

	return nil
}
