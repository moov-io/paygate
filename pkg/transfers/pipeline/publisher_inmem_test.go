// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"testing"
)

func TestPublisher__testing(t *testing.T) {
	pub := testingPublisher(t)
	t.Logf("pub=%#v", pub)
}
