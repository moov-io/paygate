// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package util

import (
	"strconv"
	"strings"
)

// Or returns primary if non-empty and backup otherwise
func Or(primary, backup string) string {
	primary = strings.TrimSpace(primary)
	if primary == "" {
		return strings.TrimSpace(backup)
	}
	return primary
}

// Yes returns true if the provided case-insensitive string matches 'yes' and is used to parse config values.
func Yes(in string) bool {
	in = strings.TrimSpace(in)
	if strings.EqualFold(in, "yes") {
		return true
	}
	v, _ := strconv.ParseBool(in)
	return v
}
