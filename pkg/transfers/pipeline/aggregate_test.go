// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moov-io/paygate/pkg/upload"

	"github.com/go-kit/kit/log"

	"github.com/moov-io/paygate/pkg/transfers/pipeline/notify"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"gocloud.dev/pubsub"
)

func TestAggregate__handleMessageXfer(t *testing.T) {
	pub := testingPublisher(t)
	sub := testingSubscriber(t, pub)

	merge := &MockXferMerging{}

	file, _ := ach.ReadFile(filepath.Join("..", "..", "..", "testdata", "ppd-debit.ach"))
	err := pub.Upload(Xfer{
		Transfer: &client.Transfer{
			TransferID: "transfer-id",
		},
		File: file,
	})
	if err != nil {
		t.Fatal(err)
	}

	msg, err := sub.Receive(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if err := handleMessage(merge, msg); err != nil {
		t.Fatal(err)
	}

	if merge.LatestXfer == nil {
		t.Fatal("missing merge.LatestXfer")
	}
	if merge.LatestXfer.Transfer.TransferID != "transfer-id" {
		t.Errorf("unexpected %#v", merge.LatestXfer)
	}
}

func TestAggregate__handleMessageCancel(t *testing.T) {
	pub := testingPublisher(t)
	sub := testingSubscriber(t, pub)

	merge := &MockXferMerging{}

	err := pub.Cancel(CanceledTransfer{
		TransferID: base.ID(),
	})
	if err != nil {
		t.Fatal(err)
	}

	msg, err := sub.Receive(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if err := handleMessage(merge, msg); err != nil {
		t.Fatal(err)
	}

	if merge.LatestCancel == nil {
		t.Fatal("missing merge.LatestCancel")
	}
}

func TestAggregate__handleMessageErr(t *testing.T) {
	merge := &MockXferMerging{}
	msg := &pubsub.Message{
		Body: []byte("unexpected message"),
	}

	if err := handleMessage(merge, msg); err == nil {
		t.Error("expected error")
	}
}

func TestAggregate_notifyAfterUpload(t *testing.T) {
	mockNotifier := &notify.MockSender{}
	xferAggregator := &XferAggregator{
		agent:    &upload.MockAgent{},
		notifier: mockNotifier,
		logger:   log.NewNopLogger(),
	}

	require.NotPanics(t, func() {
		xferAggregator.notifyAfterUpload("filename.txt", nil, nil)
	})
	require.True(t, mockNotifier.InfoWasCalled())
	require.False(t, mockNotifier.CriticalWasCalled())
	require.NotEmpty(t, mockNotifier.CapturedMessage())
	require.NotEmpty(t, mockNotifier.CapturedMessage().Hostname)
}

func TestAggregate_notifyAfterUploadErr(t *testing.T) {
	mockNotifier := &notify.MockSender{}
	xferAggregator := &XferAggregator{
		agent:    &upload.MockAgent{},
		notifier: mockNotifier,
		logger:   log.NewNopLogger(),
	}

	require.NotPanics(t, func() {
		xferAggregator.notifyAfterUpload("filename.txt", nil, errors.New("upload failed"))
	})
	require.False(t, mockNotifier.InfoWasCalled())
	require.True(t, mockNotifier.CriticalWasCalled())
	require.NotEmpty(t, mockNotifier.CapturedMessage())
	require.NotEmpty(t, mockNotifier.CapturedMessage().Hostname)
}
