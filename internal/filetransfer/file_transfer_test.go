// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/goftp/server"
)

func TestCutoffTime(t *testing.T) {
	now := time.Now()
	loc, _ := time.LoadLocation("America/New_York")
	ct := &CutoffTime{RoutingNumber: "123456789", Cutoff: 1700, Loc: loc}

	// before
	when := time.Date(now.Year(), now.Month(), now.Day(), 12, 34, 0, 0, loc)
	if d := ct.Diff(when); d != (4*time.Hour)+(26*time.Minute) { // written at 4:37PM
		t.Errorf("got %v", d)
	}

	// 1min before
	when = time.Date(now.Year(), now.Month(), now.Day(), 16, 59, 0, 0, loc)
	if d := ct.Diff(when); d != 1*time.Minute { // written at 4:38PM
		t.Errorf("got %v", d)
	}

	// 1min after
	when = time.Date(now.Year(), now.Month(), now.Day(), 17, 01, 0, 0, loc)
	if d := ct.Diff(when); d != -1*time.Minute { // written at 4:38PM
		t.Errorf("got %v", d)
	}

	// after
	when = time.Date(now.Year(), now.Month(), now.Day(), 18, 21, 0, 0, loc)
	if d := ct.Diff(when); d != (-1*time.Hour)-(21*time.Minute) { // written at 4:40PM
		t.Errorf("got %v", d)
	}
}

func TestCutoffTime__JSON(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	ct := &CutoffTime{RoutingNumber: "123456789", Cutoff: 1700, Loc: loc}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(ct); err != nil {
		t.Fatal(err)
	}

	// Crude check of JSON properties
	if !strings.Contains(buf.String(), `"RoutingNumber":"123456789"`) {
		t.Error(buf.String())
	}
	if !strings.Contains(buf.String(), `"Cutoff":1700`) {
		t.Error(buf.String())
	}
	if !strings.Contains(buf.String(), `"Location":"America/New_York"`) {
		t.Error(buf.String())
	}
}

func createTestFileTransferAgent(t *testing.T) (*server.Server, Agent) {
	svc, err := createTestFTPServer(t)
	if err != nil {
		return nil, nil
	}

	auth, ok := svc.Auth.(*server.SimpleAuth)
	if !ok {
		t.Errorf("unknown svc.Auth: %T", svc.Auth)
	}
	conf := &Config{ // these need to match paths at testdata/ftp-srever/
		InboundPath:  "inbound",
		OutboundPath: "outbound",
		ReturnPath:   "returned",
	}
	ftpConfigs := []*FTPConfig{
		{
			Hostname: fmt.Sprintf("%s:%d", svc.Hostname, svc.Port),
			Username: auth.Name,
			Password: auth.Password,
		},
	}
	agent, err := newFTPTransferAgent(conf, ftpConfigs)
	if err != nil {
		svc.Shutdown()
		t.Fatalf("problem creating FileTransferAgent: %v", err)
		return nil, nil
	}
	return svc, agent
}
