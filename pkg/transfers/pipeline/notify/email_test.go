// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

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
	cfg := &config.Email{
		CompanyName: "Moov",
	}
	msg := &Message{
		Direction: Upload,
		Filename:  "20200529-131400.ach",
	}
	if f, err := ach.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "ppd-debit.ach")); err != nil {
		t.Fatal(err)
	} else {
		msg.File = f
	}

	contents, err := marshalEmail(cfg, msg)
	if err != nil {
		t.Fatal(err)
	}

	if testing.Verbose() {
		t.Log(contents)
	}

	if !strings.Contains(contents, `A file has been uploaded from Moov: 20200529-131400.ach`) {
		t.Error("generated template doesn't match")
	}
	if !strings.Contains(contents, `Debits:  $105.00`) {
		t.Error("generated template doesn't match")
	}
	if !strings.Contains(contents, `Credits: $0.00`) {
		t.Error("generated template doesn't match")
	}
	if !strings.Contains(contents, `Batches: 1`) {
		t.Error("generated template doesn't match")
	}
	if !strings.Contains(contents, `Total Entries: 1`) {
		t.Error("generated template doesn't match")
	}
}
