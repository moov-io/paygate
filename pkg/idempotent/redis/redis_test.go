// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package redis

import (
	"testing"
)

func TestRedis(t *testing.T) {
	redis := New()

	if redis.SeenBefore("key") {
		t.Errorf("expected not seen")
	}

	if !redis.SeenBefore("key") {
		t.Errorf("expected seen")
	}

	if redis.SeenBefore("other key") {
		t.Errorf("expected not seen")
	}
	if redis.FlushAll() != nil {
		t.Errorf("flush all error")
	}
}
