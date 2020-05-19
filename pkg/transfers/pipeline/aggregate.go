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
	"io/ioutil"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/transfers/pipeline/notify"
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
	cfg    *config.Config
	logger log.Logger

	agent    upload.Agent
	notifier notify.Sender

	merger       XferMerging
	subscription *pubsub.Subscription

	cutoffTrigger chan manuallyTriggeredCutoff
}

func NewAggregator(cfg *config.Config, agent upload.Agent, merger XferMerging, sub *pubsub.Subscription) (*XferAggregator, error) {
	notifier, err := notify.NewMultiSender(cfg.Pipeline.Notifications)
	if err != nil {
		return nil, err
	}
	return &XferAggregator{
		cfg:           cfg,
		logger:        cfg.Logger,
		agent:         agent,
		notifier:      notifier,
		merger:        merger,
		subscription:  sub,
		cutoffTrigger: make(chan manuallyTriggeredCutoff, 1),
	}, nil
}

// receive each message of *pubsub.Subscription, detect message type
//   - if Xfer, write into ./mergable/
//   - if Xfer, write/rename as ./mergable/foo.ach.deleted ?
//   - on cutoff merge files

func (xfagg *XferAggregator) Start(ctx context.Context, cutoffs *schedule.CutoffTimes) {
	for {
		select {
		case tt := <-cutoffs.C:
			xfagg.withEachFile(tt)

		case waiter := <-xfagg.cutoffTrigger:
			xfagg.manualCutoff(waiter)

		case msg := <-xfagg.await():
			if err := handleMessage(xfagg.merger, msg); err != nil {
				xfagg.logger.Log("aggregate", fmt.Sprintf("ERROR handling message: %v", err))
			}

		case <-ctx.Done():
			cutoffs.Stop()
			xfagg.Shutdown()
		}
	}
}

func (xfagg *XferAggregator) Shutdown() {
	xfagg.logger.Log("aggregate", "shutting down xfer aggregation")

	if err := xfagg.subscription.Shutdown(context.Background()); err != nil {
		xfagg.logger.Log("shutdown", fmt.Sprintf("problem shutting down transfer aggregator: %v", err))
	}
}

func (xfagg *XferAggregator) manualCutoff(waiter manuallyTriggeredCutoff) {
	xfagg.logger.Log("aggregate", "starting manual cutoff window processing")

	if err := xfagg.merger.WithEachMerged(xfagg.uploadFile); err != nil {
		waiter.C <- err
		xfagg.logger.Log("aggregate", fmt.Sprintf("ERROR inside manual WithEachMerged: %v", err))
	} else {
		waiter.C <- nil
	}

	xfagg.logger.Log("aggregate", "ended manual cutoff window processing")
}

func (xfagg *XferAggregator) withEachFile(when time.Time) {
	window := when.Format("15:04")
	xfagg.logger.Log("aggregate", fmt.Sprintf("starting %s cutoff window processing", window))

	// TODO(adam): need a step here for GPG encryption, balancing, etc of files

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
	filename, err := upload.RenderACHFilename(xfagg.cfg.ODFI.FilenameTemplate(), data)
	if err != nil {
		return fmt.Errorf("problem rendering filename template: %v", err)
	}

	var buf bytes.Buffer
	if err := ach.NewWriter(&buf).Write(f); err != nil {
		return fmt.Errorf("unable to buffer ACH file: %v", err)
	}

	// Upload our file
	err = xfagg.agent.UploadFile(upload.File{
		Filename: filename,
		Contents: ioutil.NopCloser(&buf),
	})

	// Send Slack/PD or whatever notifications after the file is uploaded
	xfagg.notifyAfterUpload(filename, err)

	return err
}

func (xfagg *XferAggregator) notifyAfterUpload(filename string, err error) {
	body := fmt.Sprintf("upload of %s", filename)
	if err != nil {
		msg := &notify.Message{
			Body: "failed to " + body,
		}
		if err := xfagg.notifier.Critical(msg); err != nil {
			xfagg.logger.Log("problem sending critical notification for file=%s: %v", filename, err)
		}
	} else {
		msg := &notify.Message{
			Body: "successful " + body,
		}
		if err := xfagg.notifier.Info(msg); err != nil {
			xfagg.logger.Log("problem sending info notification for file=%s: %v", filename, err)
		}
	}
}

func (xfagg *XferAggregator) await() chan *pubsub.Message {
	out := make(chan *pubsub.Message, 1)
	go func() {
		msg, err := xfagg.subscription.Receive(context.Background())
		if err != nil {
			xfagg.logger.Log("aggregate", fmt.Sprintf("ERROR receiving message: %v", err))
		}
		out <- msg
	}()
	return out
}

// handleMessage attempts to parse a pubsub.Message into a strongly typed message
// which an XferMerging instance can handle.
func handleMessage(merger XferMerging, msg *pubsub.Message) error {
	if msg == nil {
		return errors.New("nil pubsub.Message")
	}

	var xfer Xfer
	err := json.NewDecoder(bytes.NewReader(msg.Body)).Decode(&xfer)
	if err == nil && xfer.Transfer != nil && xfer.File != nil {
		msg.Ack()
		// Handle the Xfer after decoding it.
		if err := merger.HandleXfer(xfer); err != nil {
			if msg.Nackable() {
				msg.Nack()
			}
			return fmt.Errorf("HandleXfer problem with transferID=%s: %v", xfer.Transfer.TransferID, err)
		}
		return nil
	}

	var cancel CanceledTransfer
	if err := json.NewDecoder(bytes.NewReader(msg.Body)).Decode(&cancel); err == nil && cancel.TransferID != "" {
		msg.Ack()
		// Cancel the given transfer
		if err := merger.HandleCancel(cancel); err != nil {
			if msg.Nackable() {
				msg.Nack()
			}
			return fmt.Errorf("CanceledTransfer problem with transferID=%s: %v", cancel.TransferID, err)
		}
		return nil
	}

	if msg.Nackable() {
		msg.Nack()
	}

	return fmt.Errorf("unexpected message: %v", string(msg.Body))
}
