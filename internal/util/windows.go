// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package util

import (
	"os"
	"strings"
	"testing"
)

func SkipInsideWindowsCI(t *testing.T) {
	if strings.EqualFold(os.Getenv("TRAVIS_OS_NAME"), "windows") {
		t.Skip("Docker disabled on Windows TravisCI windows builds")
	}
}
