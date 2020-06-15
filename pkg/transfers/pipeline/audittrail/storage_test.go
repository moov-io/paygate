// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package audittrail

import (
	"testing"

	"github.com/moov-io/paygate/pkg/config"
)

func TestStorageErr(t *testing.T) {
	if store, err := NewStorage(nil); store == nil || err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if store, err := NewStorage(&config.AuditTrail{}); store != nil || err == nil {
		t.Errorf("unexpected store: %v", store)
	}
}
