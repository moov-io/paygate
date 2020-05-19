// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal"
)

func TestMerging__getNonCanceledMatches(t *testing.T) {
	dir := internal.TestDir(t)

	write := func(filename string) string {
		err := ioutil.WriteFile(filepath.Join(dir, filename), nil, 0644)
		if err != nil {
			t.Fatal(err)
		}
		return filename
	}

	transfer := write(fmt.Sprintf("%s.ach", base.ID()))
	canceled := write(fmt.Sprintf("%s.ach", base.ID()))
	canceled = write(fmt.Sprintf("%s.canceled", canceled))

	matches, err := getNonCanceledMatches(filepath.Join(dir, "*.ach"))
	if err != nil {
		t.Fatal(err)
	}

	if len(matches) != 1 {
		t.Errorf("got %d matches: %v", len(matches), matches)
	}
	if !strings.HasSuffix(matches[0], transfer) {
		t.Errorf("unexpected match: %v", matches[0])
	}
	if strings.Contains(matches[0], canceled) {
		t.Errorf("unexpected match: %v", matches[0])
	}
}
