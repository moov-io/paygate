// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package util

import (
	"testing"
	"time"
)

func TestTimeout(t *testing.T) {
	start := time.Now()

	err := Timeout(func() error {
		time.Sleep(50 * time.Millisecond)
		return nil
	}, 1*time.Second)

	if err != nil {
		t.Fatal(err)
	}

	diff := time.Since(start)

	if diff < 50*time.Millisecond {
		t.Errorf("%v was under 50ms", diff)
	}
	if limit := 2 * 100 * time.Millisecond; diff > limit {
		t.Errorf("%v was over %v", diff, limit)
	}
}
