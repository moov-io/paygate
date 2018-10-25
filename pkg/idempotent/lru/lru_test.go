// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package lru

import (
	"testing"
)

func TestLRU(t *testing.T) {
	cache := New()

	if seen, _ := cache.SeenBefore("key"); seen {
		t.Errorf("expected not seen")
	}

	if seen, _ := cache.SeenBefore("key"); !seen {
		t.Errorf("expected seen")
	}

	if seen, _ := cache.SeenBefore("other key"); seen {
		t.Errorf("expected not seen")
	}
}
