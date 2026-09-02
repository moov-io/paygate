// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package upload

import (
	"net"
	"strings"
	"testing"

	"github.com/moov-io/paygate/pkg/config"
)

func TestRejectOutboundIPRange(t *testing.T) {
	addrs, err := net.LookupIP("moov.io")
	if err != nil {
		t.Fatal(err)
	}

	var ipv4s []net.IP
	for i := range addrs {
		if a := addrs[i].To4(); a != nil {
			ipv4s = append(ipv4s, a)
		}
	}
	if len(ipv4s) == 0 {
		t.Fatal("no IPv4 addresses resolved for moov.io")
	}

	// moov.io is behind Cloudflare and returns multiple A records in
	// adjacent /24s (and IPv6). Whitelist every IPv4 so DNS order cannot flake.
	exact := make([]string, len(ipv4s))
	var cidrs []string
	seenCIDR := make(map[string]struct{})
	for i := range ipv4s {
		exact[i] = ipv4s[i].String()
		c := ipv4s[i].Mask(net.IPv4Mask(0xFF, 0xFF, 0xFF, 0x0)).String() + "/24"
		if _, ok := seenCIDR[c]; !ok {
			seenCIDR[c] = struct{}{}
			cidrs = append(cidrs, c)
		}
	}

	cfg := &config.ODFI{AllowedIPs: strings.Join(exact, ",")}

	// exact IP match
	if err := rejectOutboundIPRange(cfg.SplitAllowedIPs(), "moov.io"); err != nil {
		t.Error(err)
	}

	// multiple whitelisted, but exact IP match
	cfg.AllowedIPs = "127.0.0.1/24," + strings.Join(exact, ",")
	if err := rejectOutboundIPRange(cfg.SplitAllowedIPs(), "moov.io"); err != nil {
		t.Error(err)
	}

	// match each resolved address's /24 (Cloudflare IPs are not in one /24)
	cfg.AllowedIPs = strings.Join(cidrs, ",")
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
