// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package schedule

import (
	"testing"
	"time"
)

func TestCutoffTimes(t *testing.T) {
	if testing.Short() {
		t.Skip("this test can take up to 60s, skipping")
	}

	next := time.Now().Add(time.Minute).Format("15:04")

	cutoffs, err := ForCutoffTimes(time.Local.String(), []string{next})
	if err != nil {
		t.Fatal(err)
	}
	defer cutoffs.Stop()

	tt := <-cutoffs.C // block on channel read

	expected := tt.Format("15:04")
	if next != expected {
		t.Errorf("next=%q expected=%q", next, expected)
	}
}

func TestCutoffTimesErr(t *testing.T) {
	_, err := ForCutoffTimes("bad_zone", nil)
	if err == nil {
		t.Error("expected error")
	}
	_, err = ForCutoffTimes(time.Local.String(), nil)
	if err == nil {
		t.Error("expected error")
	}
	_, err = ForCutoffTimes(time.Local.String(), []string{"bad:time"})
	if err == nil {
		t.Error("expected error")
	}
}
