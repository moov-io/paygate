// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package upload

import (
	"fmt"
	"net"
	"strings"

	"github.com/moov-io/paygate/internal/filetransfer/config"
)

func rejectOutboundIPRange(cfg *config.Config, hostname string) error {
	if cfg.AllowedIPs == "" {
		return nil
	}

	addrs, err := net.LookupIP(hostname)
	if len(addrs) == 0 || err != nil {
		return fmt.Errorf("unable to resolve (found %d) %s: %v", len(addrs), hostname, err)
	}

	ips := strings.Split(cfg.AllowedIPs, ",")
	for i := range ips {
		if strings.Contains(ips[i], "/") {
			ip, ipnet, err := net.ParseCIDR(ips[i])
			if err != nil {
				return err
			}
			if ip.Equal(addrs[0]) || ipnet.Contains(addrs[0]) {
				return nil // whitelisted
			}
		} else {
			if net.ParseIP(ips[i]).Equal(addrs[0]) {
				return nil // whitelisted
			}
		}
	}
	return fmt.Errorf("%s is not whitelisted", addrs[0].String())
}
