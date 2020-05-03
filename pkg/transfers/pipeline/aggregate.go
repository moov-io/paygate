// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"context"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/upload"

	"gocloud.dev/pubsub"
)

// XferAggregator ...
//
// this has a for loop which is triggered on cutoff warning
//  e.g. 10mins before 30mins before cutoff (10 mins is Moov's window, 30mins is ODFI)
// consume as many transfers as possible, then upload.
type XferAggregator struct {
	agent upload.Agent

	merger       XferMerging
	subscription *pubsub.Subscription
}

// receive each message of *pubsub.Subscription, detect message type
//   - if Xfer, write into ./mergable/
//   - if Xfer, write/rename as ./mergable/foo.ach.deleted ?
//   - on cutoff merge files

func (xfagg *XferAggregator) Start(ctx context.Context) error {
	// when <-ctx.Done() fires shutdown xfagg.consumer and upload any files we have

	var cutoff time.Ticker

	incomingMessage := make(chan *pubsub.Message, 1)

	for {
		select {
		case <-cutoff.C: // TODO(adam): merge files and upload
			if err := xfagg.merger.WithEachMerged(xfagg.uploadFile); err != nil {
				// TODO(adam): log or something
			}

		case msg := <-incomingMessage:
			if err := handleMessage(xfagg.merger, msg); err != nil {
				// TODO(adam): log or something
			}

		case <-ctx.Done():
			// TODO(adam): shutdown?
		}
	}

	return nil
}

func (xfagg *XferAggregator) uploadFile(f *ach.File) error {
	// TODO(adam): some sort of filename_template.go interaction here

	return xfagg.agent.UploadFile(upload.File{
		Filename: "",
		// Contents: io.ReadCloser,
	})
}
