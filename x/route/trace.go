// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package route

import (
	"fmt"
	"strings"

	"github.com/moov-io/paygate/x/trace"

	opentracing "github.com/opentracing/opentracing-go"
)

func (r *Responder) Span() opentracing.Span {
	method := strings.ToLower(r.request.Method)
	path := CleanPath(r.request.URL.Path)

	name := fmt.Sprintf("%s-%s", method, path)

	return trace.FromRequest(name, r.request)
}
