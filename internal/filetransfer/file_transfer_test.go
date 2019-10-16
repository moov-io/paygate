// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestConfig__outboundFilenameTemplate(t *testing.T) {
	var cfg *Config
	if tmpl := cfg.outboundFilenameTemplate(); tmpl != defaultFilenameTemplate {
		t.Errorf("expected default template: %v", tmpl)
	}

	cfg = &Config{
		OutboundFilenameTemplate: `{{ date "20060102" }}`,
	}
	if tmpl := cfg.outboundFilenameTemplate(); tmpl == defaultFilenameTemplate {
		t.Errorf("expected custom template: %v", tmpl)
	}
}

func TestCutoffTime(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Now().In(loc)
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
