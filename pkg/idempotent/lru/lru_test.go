// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package lru

import (
	"testing"
)

func TestLRU(t *testing.T) {
	cache := New()

	if cache.SeenBefore("key") {
		t.Errorf("expected not seen")
	}

	if !cache.SeenBefore("key") {
		t.Errorf("expected seen")
	}

	if cache.SeenBefore("other key") {
		t.Errorf("expected not seen")
	}
}
