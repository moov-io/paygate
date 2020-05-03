// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"testing"
)

type inmemPublisher struct{}

// use t.Name() as 'mem://<topic>'

func createMemoryPublisher(t *testing.T) *inmemPublisher {
	// 'mem://moov'
	return nil
}

// type inmemConsumer struct{}

// func createMemoryConsumer(t *testing.T) *inmemConsumer {
// 	// 'mem://moov'
// 	return nil
// }
