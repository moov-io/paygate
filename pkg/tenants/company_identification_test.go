// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package tenants

import (
	"strings"
	"testing"
)

func TestCompanyIdentification(t *testing.T) {
	id := CompanyIdentification("MOOV")
	if len(id) != 10 {
		t.Errorf("got %q", id)
	}
	if !strings.HasPrefix(id, "MOOV") {
		t.Errorf("unexpected %q", id)
	}
}
