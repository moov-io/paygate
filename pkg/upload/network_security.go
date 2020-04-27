// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package upload

import (
	"fmt"
	"net"
	"strings"
)

func rejectOutboundIPRange(allowedIPs []string, hostname string) error {
	// perform an initial check to see if we can resolve the hostname
	if strings.Contains(hostname, ":") {
		if host, _, err := net.SplitHostPort(hostname); err != nil {
			return err
		} else {
			hostname = host
		}
	}
	addrs, err := net.LookupIP(hostname)
	if len(addrs) == 0 || err != nil {
		return fmt.Errorf("unable to resolve (found %d) %s: %v", len(addrs), hostname, err)
	}
	// skip whitelist check if none were specified, assume it was empty in the config
	if len(allowedIPs) == 0 {
		return nil
	}
	for i := range allowedIPs {
		if strings.Contains(allowedIPs[i], "/") {
			ip, ipnet, err := net.ParseCIDR(allowedIPs[i])
			if err != nil {
				return err
			}
			if ip.Equal(addrs[0]) || ipnet.Contains(addrs[0]) {
				return nil // whitelisted
			}
		} else {
			if net.ParseIP(allowedIPs[i]).Equal(addrs[0]) {
				return nil // whitelisted
			}
		}
	}
	return fmt.Errorf("%s is not whitelisted", addrs[0].String())
}
