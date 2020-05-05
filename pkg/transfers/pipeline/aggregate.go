// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/upload"
	"github.com/moov-io/paygate/x/schedule"

	"github.com/go-kit/kit/log"
	"gocloud.dev/pubsub"
)

// XferAggregator ...
//
// this has a for loop which is triggered on cutoff warning
//  e.g. 10mins before 30mins before cutoff (10 mins is Moov's window, 30mins is ODFI)
// consume as many transfers as possible, then upload.
type XferAggregator struct {
	cfg    config.ODFI
	logger log.Logger

	agent upload.Agent

	merger       XferMerging
	subscription *pubsub.Subscription
}

func NewAggregator(logger log.Logger, cfg config.ODFI, agent upload.Agent, merger XferMerging, sub *pubsub.Subscription) *XferAggregator {
	return &XferAggregator{
		cfg:          cfg,
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

	cutoffs, err := schedule.ForCutoffTimes(xfagg.cfg.Cutoffs.Timezone, xfagg.cfg.Cutoffs.Windows)
	if err != nil {
		return fmt.Errorf("problem with cutoff times: %v", err)
	}
	xfagg.logger.Log("aggregate", fmt.Sprintf("registered %s cutoffs=%v", xfagg.cfg.Cutoffs.Timezone, strings.Join(xfagg.cfg.Cutoffs.Windows, ",")))

	for {
		select {
		case tt := <-cutoffs.C:
			xfagg.withEachFile(tt)

		case msg := <-xfagg.await():
			if err := handleMessage(xfagg.merger, msg); err != nil {
				xfagg.logger.Log("aggregate", fmt.Sprintf("ERROR handling message: %v", err))
			}

		case <-ctx.Done():
			xfagg.logger.Log("aggregate", "shutting down xfer aggregation")
			cutoffs.Stop()
		}
	}

	return nil
}

func (xfagg *XferAggregator) withEachFile(when time.Time) {
	window := when.Format("15:04")
	xfagg.logger.Log("aggregate", fmt.Sprintf("starting %s cutoff window processing", window))

	if err := xfagg.merger.WithEachMerged(xfagg.uploadFile); err != nil {
		xfagg.logger.Log("aggregate", fmt.Sprintf("ERROR inside WithEachMerged: %v", err))
	}

	xfagg.logger.Log("aggregate", fmt.Sprintf("ended %s cutoff window processing", window))
}

func (xfagg *XferAggregator) uploadFile(f *ach.File) error {
	data := upload.FilenameData{
		RoutingNumber: f.Header.ImmediateDestination,
		N:             "1", // TODO(adam): upload.ACHFilenameSeq(..) we need to increment sequence number
	}
	filename, err := upload.RenderACHFilename(xfagg.cfg.FilenameTemplate(), data)
	if err != nil {
		return fmt.Errorf("problem rendering filename template: %v", err)
	}

	var buf bytes.Buffer
	if err := ach.NewWriter(&buf).Write(f); err != nil {
		return fmt.Errorf("unable to buffer ACH file: %v", err)
	}

	return xfagg.agent.UploadFile(upload.File{
		Filename: filename,
		Contents: ioutil.NopCloser(&buf),
	})
}

func (xfagg *XferAggregator) await() chan *pubsub.Message {
	out := make(chan *pubsub.Message, 1)
	go func() {
		msg, err := xfagg.subscription.Receive(context.Background())
		if err != nil {
			xfagg.logger.Log("aggregate", fmt.Sprintf("ERROR receiving message: %v", err))
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
