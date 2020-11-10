// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

import (
	"errors"
	"testing"

	"github.com/moov-io/paygate/pkg/config"

	"github.com/stretchr/testify/require"
)

func TestMultiSender(t *testing.T) {
	cfg := config.Empty()
	sender, err := NewMultiSender(cfg.Logger, cfg.Pipeline.Notifications)
	if err != nil {
		t.Fatal(err)
	}

	msg := &Message{Direction: Upload}

	require.NoError(t, sender.Info(msg))
	require.NoError(t, sender.Critical(msg))

	sender.senders = append(sender.senders, &MockSender{})

	require.NoError(t, sender.Info(msg))
	require.NoError(t, sender.Critical(msg))
}

func TestMultiSenderErr(t *testing.T) {
	sendErr := errors.New("bad error")

	cfg := config.Empty()
	sender := &MultiSender{
		logger: cfg.Logger,
		senders: []Sender{
			&MockSender{Err: sendErr},
		},
	}

	msg := &Message{Direction: Upload}

	require.Equal(t, sender.Info(msg), sendErr)
	require.Equal(t, sender.Critical(msg), sendErr)
}
