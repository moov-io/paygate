// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"gocloud.dev/pubsub"
)

func handleMessage(merger XferMerging, msg *pubsub.Message) error {
	// TODO(adam): type switch on parsed msg.Body type

	// merger.HandleXfer(Xfer) error
	// merger.HandleCancel(Xfer) error

	return nil
}
