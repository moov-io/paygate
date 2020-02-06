// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"testing"
)

func TestACH__trackError(t *testing.T) {
	achClient, _, server := MockClientServer("trackError", AddPingErrorRoute)
	defer server.Close()

	achClient.trackError("localhost:8080")
	achClient.trackError("http://localhost:8080")
}
