// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.
package route

import (
	"net/http"

	"github.com/gorilla/mux"
)

func ReadPathID(name string, r *http.Request) string {
	vars := mux.Vars(r)
	v, ok := vars[name]
	if ok {
		return v
	}
	return ""
}
