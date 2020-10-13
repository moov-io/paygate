// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/config"
)

func TestEmailSend(t *testing.T) {
	dep := spawnMailslurp(t)

	cfg := &config.Email{
		From: "noreply@moov.io",
		To: []string{
			"jane@company.com",
		},
		ConnectionURI: fmt.Sprintf("smtps://test:test@localhost:%s/?insecure_skip_verify=true", dep.SMTPPort()),
		CompanyName:   "Moov",
	}

	dialer, err := setupGoMailClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Enable SSL for our test container, this breaks if set for production SMTP server.
	// GMail fails to connect if we set this.
	dialer.SSL = strings.HasPrefix(cfg.ConnectionURI, "smtps://")

	msg := &Message{
		Direction: Upload,
		Filename:  "20200529-131400.ach",
		File:      ach.NewFile(),
	}

	body, err := marshalEmail(cfg, msg)
	if err != nil {
		t.Fatal(err)
	}

	if err := sendEmail(cfg, dialer, msg.Filename, body); err != nil {
		t.Fatal(err)
	}

	dep.Close() // remove container after successful tests
}

func TestEmail__marshal(t *testing.T) {

	f, err := ach.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		desc      string
		msg       *Message
		firstLine string
	}{
		{"upload with hostname", &Message{Direction: Upload, File: f, Filename: "20200529-131400.ach", Hostname: "138.34.204.3"},
			"A file has been uploaded to 138.34.204.3 from Moov: 20200529-131400.ach"},
		{"upload with no hostname", &Message{Direction: Upload, File: f, Filename: "20200529-131400.ach"},
			"A file has been uploaded from Moov: 20200529-131400.ach"},
		{"download ignores hostname", &Message{Direction: Download, File: f, Filename: "20200529-131400.ach", Hostname: "138.34.204.3"},
			"A file has been downloaded from Moov: 20200529-131400.ach"},
		{"download", &Message{Direction: Download, File: f, Filename: "20200529-131400.ach"},
			"A file has been downloaded from Moov: 20200529-131400.ach"},
	}

	cfg := &config.Email{
		CompanyName: "Moov",
	}

	for _, test := range tests {
		contents, err := marshalEmail(cfg, test.msg)
		if err != nil {
			t.Fatal(err)
		}

		if testing.Verbose() {
			t.Log(contents)
		}

		require.Contains(t, contents, test.firstLine)
		require.Contains(t, contents, `Debits:  $105.00`)
		require.Contains(t, contents, `Credits: $0.00`)
		require.Contains(t, contents, `Batches: 1`)
		require.Contains(t, contents, `Total Entries: 1`)
	}
}
