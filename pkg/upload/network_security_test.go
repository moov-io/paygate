// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package upload

import (
	"fmt"
	"net"
	"testing"

	"github.com/moov-io/paygate/pkg/config"
)

func TestRejectOutboundIPRange(t *testing.T) {
	addrs, err := net.LookupIP("moov.io")
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.ODFI{AllowedIPs: addrs[0].String()}

	// exact IP match
	if err := rejectOutboundIPRange(cfg.SplitAllowedIPs(), "moov.io"); err != nil {
		t.Error(err)
	}

	// multiple whitelisted, but exact IP match
	cfg.AllowedIPs = fmt.Sprintf("127.0.0.1/24,%s", addrs[0].String())
	if err := rejectOutboundIPRange(cfg.SplitAllowedIPs(), "moov.io"); err != nil {
		t.Error(err)
	}

	// multiple whitelisted, match range (convert IP to /24)
	cfg.AllowedIPs = fmt.Sprintf("%s/24", addrs[0].Mask(net.IPv4Mask(0xFF, 0xFF, 0xFF, 0x0)).String())
	if err := rejectOutboundIPRange(cfg.SplitAllowedIPs(), "moov.io"); err != nil {
		t.Error(err)
	}

	// no match
	cfg.AllowedIPs = "8.8.8.0/24"
	if err := rejectOutboundIPRange(cfg.SplitAllowedIPs(), "moov.io"); err == nil {
		t.Error("expected error")
	}

	// empty whitelist, allow all
	cfg.AllowedIPs = ""
	if err := rejectOutboundIPRange(cfg.SplitAllowedIPs(), "moov.io"); err != nil {
		t.Errorf("expected no error: %v", err)
	}

	// error cases
	cfg.AllowedIPs = "afkjsafkjahfa"
	if err := rejectOutboundIPRange(cfg.SplitAllowedIPs(), "moov.io"); err == nil {
		t.Error("expected error")
	}
	cfg.AllowedIPs = "10.0.0.0/8"
	if err := rejectOutboundIPRange(cfg.SplitAllowedIPs(), "lsjafkshfaksjfhas"); err == nil {
		t.Error("expected error")
	}
	cfg.AllowedIPs = "10...../8"
	if err := rejectOutboundIPRange(cfg.SplitAllowedIPs(), "moov.io"); err == nil {
		t.Error("expected error")
	}
}
