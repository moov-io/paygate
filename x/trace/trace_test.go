// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package trace

import (
	"testing"

	"github.com/moov-io/base/log"
	"github.com/opentracing/opentracing-go"
)

func TestConstantTracing(t *testing.T) {
	tracer, closer, err := NewConstantTracer(log.NewNopLogger(), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closer.Close() })

	// quick test
	createParentWithChild(tracer)
}

func TestProbabilisticTracing(t *testing.T) {
	tracer, closer, err := NewProbabilisticTracer(log.NewNopLogger(), "test", 0.5)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closer.Close() })

	// quick test
	createParentWithChild(tracer)
}

func createParentWithChild(tracer opentracing.Tracer) {
	parent := tracer.StartSpan("say-hello")

	child := tracer.StartSpan("child", opentracing.ChildOf(parent.Context()))
	child.Finish()

	parent.Finish()
}
