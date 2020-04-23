// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"testing"

	"github.com/moov-io/base"
)

func TestACH__trackError(t *testing.T) {
	achClient, _, server := MockClientServer("trackError", AddPingErrorRoute, AddInvalidRoute)
	defer server.Close()

	if err := achClient.Ping(); err == nil {
		t.Error("expected error")
	}
	if err := achClient.ValidateFile(base.ID()); err == nil {
		t.Error("expected error")
	}

	achClient.trackError("ping")

	achClient.endpoint = "localhost"
	achClient.trackError("ping")

	achClient.endpoint = "invalid hostname"
	achClient.trackError("ping")
}
