// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package trace

import (
	"net/http"
	"testing"

	"github.com/moov-io/base/log"
	"github.com/uber/jaeger-client-go"
)

func TestDecorateHttpRequest(t *testing.T) {
	tracer, closer, err := NewConstantTracer(log.NewNopLogger(), "http-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closer.Close() })

	span := tracer.StartSpan("service-ping")
	defer span.Finish()

	req, _ := http.NewRequest("GET", "/ping", nil)
	req = DecorateHttpRequest(req, span)

	if v := req.Header.Get(jaeger.TraceContextHeaderName); v == "" {
		t.Errorf("missing trace header: %#v", req.Header)
	}
}

func TestFromRequest(t *testing.T) {
	_, closer, err := NewConstantTracer(log.NewNopLogger(), "http-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closer.Close() })

	req, _ := http.NewRequest("GET", "/ping", nil)

	// no incoming trace header, so expect no header
	span := FromRequest("service-ping", req)
	if span == nil {
		t.Fatal("nil Span")
	}
	if v := req.Header.Get(jaeger.TraceContextHeaderName); v != "" {
		t.Errorf("expected trace header: %#v", req.Header)
	}

	// even with an empty tracer expect requests can be decorated
	req = DecorateHttpRequest(req, FromRequest("service-ping", req))

	h1 := req.Header.Get(jaeger.TraceContextHeaderName)
	if h1 == "" {
		t.Errorf("expected trace header: %#v", req.Header)
	}

	req2, _ := http.NewRequest("DELETE", "/foo/id", nil)
	req2 = DecorateHttpRequest(req2, FromRequest("removal", req))

	h2 := req2.Header.Get(jaeger.TraceContextHeaderName)
	if h2 == "" {
		t.Errorf("expected trace header: %#v", req2.Header)
	}
}
