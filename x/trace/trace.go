// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package trace

import (
	"io"

	"github.com/moov-io/base/log"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/opentracing/opentracing-go"
	jaegermetrics "github.com/uber/jaeger-lib/metrics/prometheus"

	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
)

// NewConstantTracer returns an opentracer.Tracer from Jaeger that always records spans for recording.
//
// This method uses the opentracing singleton and Proometheus DefaultRegisterer singleton.
func NewConstantTracer(logger log.Logger, serviceName string) (opentracing.Tracer, io.Closer, error) {
	cfg := jaegercfg.Configuration{
		ServiceName: serviceName,
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1.0,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans: true,
		},
	}
	return setupTracer(logger, cfg)
}

// NewProbabilisticTracer returns an opentracer.Tracer from Jaeger that records approximately
// the given percentage of spans for recording.
//
// This method uses the opentracing singleton and Proometheus DefaultRegisterer singleton.
func NewProbabilisticTracer(logger log.Logger, serviceName string, rate float64) (opentracing.Tracer, io.Closer, error) {
	cfg := jaegercfg.Configuration{
		ServiceName: serviceName,
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeProbabilistic,
			Param: rate,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans: true,
		},
	}
	return setupTracer(logger, cfg)
}

var (
	// wrappedPrometheusRegisterer is a singleton so we only register opentracing metrics once
	wrappedPrometheusRegisterer = jaegermetrics.New(jaegermetrics.WithRegisterer(prometheus.DefaultRegisterer))
)

func setupTracer(logger log.Logger, cfg jaegercfg.Configuration) (opentracing.Tracer, io.Closer, error) {
	// Initialize tracer with a logger and a metrics factory
	tracer, closer, err := cfg.NewTracer(
		jaegercfg.Logger(&jaegerLogger{inner: logger}),
		jaegercfg.Metrics(wrappedPrometheusRegisterer),
	)

	// Set the singleton opentracing.Tracer with the Jaeger tracer.
	opentracing.SetGlobalTracer(tracer)

	return tracer, closer, err
}

func GlobalTracer() opentracing.Tracer {
	return opentracing.GlobalTracer()
}

var _ jaeger.Logger = (*jaegerLogger)(nil)

// adapter for jaeger.Logger
type jaegerLogger struct {
	inner log.Logger
}

func (l *jaegerLogger) Error(msg string) {
	l.inner.LogErrorf(msg)
}

func (l *jaegerLogger) Infof(msg string, args ...interface{}) {
	l.inner.Logf(msg, args...)
}
