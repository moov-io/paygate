// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"path/filepath"
	"sort"
	"testing"
)

func TestFiles__collectTraceNumbers(t *testing.T) {
	// this file has multiple trace numbers
	file, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "return-WEB.ach"))
	if err != nil {
		t.Fatal(err)
	}

	traceNumbers := collectTraceNumbers(file)
	sort.Strings(traceNumbers)

	expected := []string{"021000029461242", "091000017611242"}

	if len(traceNumbers) != len(expected) {
		t.Errorf("got %d trace numbers expected %d", len(traceNumbers), len(expected))
	}
	for i := range traceNumbers {
		if traceNumbers[i] != expected[i] {
			t.Errorf("#%d got=%q expected=%q", i, traceNumbers[i], expected[i])
		}
	}
}
