// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package trace

import (
	"fmt"
	"net/http"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

func DecorateHttpRequest(req *http.Request, span opentracing.Span) *http.Request {
	tracer := opentracing.GlobalTracer()

	// Set some tags on the Span to annotate it.
	// The additional HTTP tags are useful for debugging purposes.
	ext.SpanKindRPCClient.Set(span)
	ext.HTTPUrl.Set(span, req.URL.String())
	ext.HTTPMethod.Set(span, req.Method)

	// Add the span's context into the request headers
	tracer.Inject(
		span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header),
	)

	return req
}

func FromRequest(name string, req *http.Request) opentracing.Span {
	tracer := opentracing.GlobalTracer()

	ctx, err := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
	fmt.Printf("error=%v\n", err)
	return tracer.StartSpan(name, ext.RPCServerOption(ctx))
}
