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
	"strings"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"

	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/transfers/pipeline/audittrail"
	"github.com/moov-io/paygate/pkg/transfers/pipeline/notify"
	"github.com/moov-io/paygate/pkg/transfers/pipeline/output"
	"github.com/moov-io/paygate/pkg/transfers/pipeline/transform"
	"github.com/moov-io/paygate/pkg/upload"
	"github.com/moov-io/paygate/x/schedule"

	"github.com/moov-io/base/log"
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

	repo Repository

	merger       XferMerging
	subscription *pubsub.Subscription

	cutoffCallbacks []CutoffCallback
	cutoffTrigger   chan manuallyTriggeredCutoff

	auditStorage          audittrail.Storage
	preuploadTransformers []transform.PreUpload
	outputFormatter       output.Formatter
}

func NewAggregator(
	cfg *config.Config,
	agent upload.Agent,
	repo Repository,
	merger XferMerging,
	sub *pubsub.Subscription,
	cutoffCallbacks []CutoffCallback,
) (*XferAggregator, error) {
	cfg.Logger = cfg.Logger.Set("service", log.String("XferAggregator"))
	notifier, err := notify.NewMultiSender(cfg.Logger, cfg.Pipeline.Notifications)
	if err != nil {
		return nil, err
	}

	auditStorage, err := audittrail.NewStorage(cfg.Pipeline.AuditTrail)
	if err != nil {
		return nil, err
	}
	cfg.Logger.Logf("setup %T audit storage", auditStorage)

	preuploadTransformers, err := transform.Multi(cfg.Logger, cfg.Pipeline.PreUpload)
	if err != nil {
		return nil, err
	}
	cfg.Logger.Logf("setup %#v pre-upload transformers", preuploadTransformers)

	outputFormatter, err := output.NewFormatter(cfg.Pipeline.Output)
	if err != nil {
		return nil, err
	}
	cfg.Logger.Logf("setup %T output formatter", outputFormatter)

	return &XferAggregator{
		cfg:                   cfg,
		logger:                cfg.Logger,
		agent:                 agent,
		notifier:              notifier,
		repo:                  repo,
		merger:                merger,
		subscription:          sub,
		cutoffCallbacks:       cutoffCallbacks,
		cutoffTrigger:         make(chan manuallyTriggeredCutoff, 1),
		auditStorage:          auditStorage,
		preuploadTransformers: preuploadTransformers,
		outputFormatter:       outputFormatter,
	}, nil
}

// CutoffCallback is a function called before cutoff processing is performed.
type CutoffCallback func() error

func (xfagg *XferAggregator) processCutoffCallbacks() error {
	if xfagg == nil || len(xfagg.cutoffCallbacks) == 0 {
		return nil
	}

	var el base.ErrorList
	for i := range xfagg.cutoffCallbacks {
		fn := xfagg.cutoffCallbacks[i]
		if fn == nil {
			continue
		}
		if err := fn(); err != nil {
			el.Add(fmt.Errorf("%T: %v", fn, err))
		}
	}

	if el.Empty() {
		return nil
	}
	return el.Err()
}

// receive each message of *pubsub.Subscription, detect message type
//   - if Xfer, write into ./mergable/
//   - if Xfer, write/rename as ./mergable/foo.ach.deleted ?
//   - on cutoff merge files

func (xfagg *XferAggregator) Start(ctx context.Context, cutoffs *schedule.CutoffTimes) {
	for {
		select {
		case tt := <-cutoffs.C:
			if err := xfagg.processCutoffCallbacks(); err != nil {
				xfagg.logger.LogErrorf("ERROR with cutoff callbacks: %v", err)
			}
			xfagg.withEachFile(tt)

		case waiter := <-xfagg.cutoffTrigger:
			if err := xfagg.processCutoffCallbacks(); err != nil {
				xfagg.logger.LogErrorf("ERROR with manual cutoff callbacks: %v", err)
			}
			xfagg.manualCutoff(waiter)

		case err := <-xfagg.await():
			if err != nil {
				xfagg.logger.LogErrorf("ERROR handling message: %v", err)
			}

		case <-ctx.Done():
			cutoffs.Stop()
			xfagg.Shutdown()
			return
		}
	}
}

func (xfagg *XferAggregator) Shutdown() {
	xfagg.logger.Log("shutting down xfer aggregation")
	if xfagg.auditStorage != nil {
		xfagg.auditStorage.Close()
	}
	if err := xfagg.subscription.Shutdown(context.Background()); err != nil {
		xfagg.logger.LogErrorf("problem shutting down transfer aggregator: %v", err)
	}
}

func (xfagg *XferAggregator) runTransformers(outgoing *ach.File) error {
	result, err := transform.ForUpload(outgoing, xfagg.preuploadTransformers)
	if err != nil {
		return err
	}
	return xfagg.uploadFile(result)
}

func (xfagg *XferAggregator) manualCutoff(waiter manuallyTriggeredCutoff) {
	xfagg.logger.Log("starting manual cutoff window processing")

	if processed, err := xfagg.merger.WithEachMerged(xfagg.runTransformers); err != nil {
		xfagg.logger.LogErrorf("ERROR inside manual WithEachMerged: %v", err)
		waiter.C <- err
	} else {
		if err := xfagg.repo.MarkTransfersAsProcessed(processed.transferIDs); err != nil {
			xfagg.logger.LogErrorf("ERROR marking %d transfers as processed: %v", len(processed.transferIDs), err)
			waiter.C <- err
		} else {
			waiter.C <- nil
		}
	}

	xfagg.logger.Log("ended manual cutoff window processing")
}

func (xfagg *XferAggregator) withEachFile(when time.Time) {
	window := when.Format("15:04")
	xfagg.logger.Logf("starting %s cutoff window processing", window)

	if processed, err := xfagg.merger.WithEachMerged(xfagg.runTransformers); err != nil {
		xfagg.logger.LogErrorf("ERROR inside WithEachMerged: %v", err)
	} else {
		if err := xfagg.repo.MarkTransfersAsProcessed(processed.transferIDs); err != nil {
			xfagg.logger.LogErrorf("ERROR marking %d transfers as processed: %v", len(processed.transferIDs), err)
		}
	}

	xfagg.logger.Logf("ended %s cutoff window processing", window)
}

func (xfagg *XferAggregator) uploadFile(res *transform.Result) error {
	if res == nil || res.File == nil {
		return errors.New("uploadFile: nil Result / File")
	}

	data := upload.FilenameData{
		RoutingNumber: res.File.Header.ImmediateDestination,
		GPG:           len(res.Encrypted) > 0,
	}
	filename, err := upload.RenderACHFilename(xfagg.cfg.ODFI.FilenameTemplate(), data)
	if err != nil {
		return fmt.Errorf("problem rendering filename template: %v", err)
	}

	var buf bytes.Buffer
	if err := xfagg.outputFormatter.Format(&buf, res); err != nil {
		return fmt.Errorf("problem formatting output: %v", err)
	}

	// Record the file in our audit trail
	if err := xfagg.auditStorage.SaveFile(filename, res.File); err != nil {
		return fmt.Errorf("problem saving file in audit record: %v", err)
	}

	// Upload our file
	err = xfagg.agent.UploadFile(upload.File{
		Filename: filename,
		Contents: ioutil.NopCloser(&buf),
	})

	// Send Slack/PD or whatever notifications after the file is uploaded
	xfagg.notifyAfterUpload(filename, res.File, err)

	return err
}

func (xfagg *XferAggregator) notifyAfterUpload(filename string, file *ach.File, err error) {
	msg := &notify.Message{
		Direction: notify.Upload,
		Filename:  filename,
		File:      file,
		Hostname:  xfagg.agent.Hostname(),
	}

	if err != nil {
		if err := xfagg.notifier.Critical(msg); err != nil {
			xfagg.logger.LogErrorf("problem sending critical notification for file=%s: %v", filename, err)
		}
	} else {
		if err := xfagg.notifier.Info(msg); err != nil {
			xfagg.logger.LogErrorf("problem sending info notification for file=%s: %v", filename, err)
		}
	}
}

func (xfagg *XferAggregator) await() chan error {
	out := make(chan error, 1)
	go func() {
		msg, err := xfagg.subscription.Receive(context.Background())
		if err != nil {
			xfagg.logger.LogErrorf("ERROR receiving message: %v", err)

			if strings.Contains(err.Error(), "has been Shutdown") {
				// quit the infinite loop if it has been shutdown
				return
			}
		}
		out <- handleMessage(xfagg.merger, msg)
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
		// Handle the Xfer after decoding it.
		if err := merger.HandleXfer(xfer); err != nil {
			if msg.Nackable() {
				msg.Nack()
			}
			return fmt.Errorf("HandleXfer problem with transferID=%s: %v", xfer.Transfer.TransferID, err)
		} else {
			msg.Ack()
		}
		return nil
	}

	var cancel CanceledTransfer
	if err := json.NewDecoder(bytes.NewReader(msg.Body)).Decode(&cancel); err == nil && cancel.TransferID != "" {
		// Cancel the given transfer
		if err := merger.HandleCancel(cancel); err != nil {
			if msg.Nackable() {
				msg.Nack()
			}
			return fmt.Errorf("CanceledTransfer problem with transferID=%s: %v", cancel.TransferID, err)
		} else {
			msg.Ack()
		}
		return nil
	}

	if msg.Nackable() {
		msg.Nack()
	}

	return fmt.Errorf("unexpected message: %v", string(msg.Body))
}
