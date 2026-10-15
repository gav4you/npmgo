// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/api 1.9.0 (build/src/metrics/Metric.js,
// metrics/NoopMeter.js, metrics/NoopMeterProvider.js and api/metrics.js).

package api

import "github.com/gav4you/npmgo/internal/jsvalue"

// ValueType is the ValueType enum: whether a metric reports integers or doubles.
type ValueType float64

// The ValueType members.
const (
	ValueType_INT    ValueType = 0
	ValueType_DOUBLE ValueType = 1
)

// MeterOptions are the options of MeterProvider.GetMeter.
type MeterOptions struct {
	SchemaUrl string
}

// MetricOptions are the options of the Meter's Create methods. Advice is the advisory configuration object
// (nil: undefined).
type MetricOptions struct {
	Description string
	Unit        string
	ValueType   ValueType
	Advice      any
}

// MeterProvider provides meters. An empty version stands for undefined.
type MeterProvider interface {
	GetMeter(name string, version string, options *MeterOptions) Meter
}

// Counter is a monotonically increasing synchronous instrument.
type Counter interface {
	Add(value float64, attributes any, context Context)
}

// UpDownCounter is a synchronous instrument that adds positive and negative values.
type UpDownCounter interface {
	Add(value float64, attributes any, context Context)
}

// Gauge records the current value.
type Gauge interface {
	Record(value float64, attributes any, context Context)
}

// Histogram records a distribution of values.
type Histogram interface {
	Record(value float64, attributes any, context Context)
}

// ObservableResult is what an observable callback reports to.
type ObservableResult interface {
	Observe(value float64, attributes any)
}

// BatchObservableResult is what a batch observable callback reports to.
type BatchObservableResult interface {
	Observe(metric Observable, value float64, attributes any)
}

// ObservableCallback is called to collect an observable's values; it returns undefined or a promise.
type ObservableCallback func(observableResult ObservableResult) any

// BatchObservableCallback is called to collect several observables' values; it returns undefined or a promise.
type BatchObservableCallback func(observableResult BatchObservableResult) any

// Observable is an asynchronous instrument.
type Observable interface {
	AddCallback(callback ObservableCallback)
	RemoveCallback(callback ObservableCallback)
}

// ObservableCounter, ObservableGauge and ObservableUpDownCounter are the asynchronous instruments.
type (
	ObservableCounter       = Observable
	ObservableGauge         = Observable
	ObservableUpDownCounter = Observable
)

// Meter creates instruments.
type Meter interface {
	CreateGauge(name string, options *MetricOptions) Gauge
	CreateHistogram(name string, options *MetricOptions) Histogram
	CreateCounter(name string, options *MetricOptions) Counter
	CreateUpDownCounter(name string, options *MetricOptions) UpDownCounter
	CreateObservableGauge(name string, options *MetricOptions) ObservableGauge
	CreateObservableCounter(name string, options *MetricOptions) ObservableCounter
	CreateObservableUpDownCounter(name string, options *MetricOptions) ObservableUpDownCounter
	AddBatchObservableCallback(callback BatchObservableCallback, observables []Observable)
	RemoveBatchObservableCallback(callback BatchObservableCallback, observables []Observable)
}

// The no-op instruments (NoopMeter.js's classes); every no-op meter returns the same instance of each.
type (
	noopMeter                         struct{}
	noopCounterMetric                 struct{}
	noopUpDownCounterMetric           struct{}
	noopGaugeMetric                   struct{}
	noopHistogramMetric               struct{}
	noopObservableCounterMetric       struct{ noopObservableMetric }
	noopObservableGaugeMetric         struct{ noopObservableMetric }
	noopObservableUpDownCounterMetric struct{ noopObservableMetric }
)

type noopObservableMetric struct{}

func (*noopObservableMetric) AddCallback(ObservableCallback)    {}
func (*noopObservableMetric) RemoveCallback(ObservableCallback) {}

func (*noopCounterMetric) Add(float64, any, Context)       {}
func (*noopUpDownCounterMetric) Add(float64, any, Context) {}
func (*noopGaugeMetric) Record(float64, any, Context)      {}
func (*noopHistogramMetric) Record(float64, any, Context)  {}

var (
	noopMeterInstance                         = &noopMeter{}
	noopCounterMetricInstance                 = &noopCounterMetric{}
	noopGaugeMetricInstance                   = &noopGaugeMetric{}
	noopHistogramMetricInstance               = &noopHistogramMetric{}
	noopUpDownCounterMetricInstance           = &noopUpDownCounterMetric{}
	noopObservableCounterMetricInstance       = &noopObservableCounterMetric{}
	noopObservableGaugeMetricInstance         = &noopObservableGaugeMetric{}
	noopObservableUpDownCounterMetricInstance = &noopObservableUpDownCounterMetric{}
)

func (*noopMeter) CreateGauge(string, *MetricOptions) Gauge { return noopGaugeMetricInstance }
func (*noopMeter) CreateHistogram(string, *MetricOptions) Histogram {
	return noopHistogramMetricInstance
}
func (*noopMeter) CreateCounter(string, *MetricOptions) Counter { return noopCounterMetricInstance }
func (*noopMeter) CreateUpDownCounter(string, *MetricOptions) UpDownCounter {
	return noopUpDownCounterMetricInstance
}
func (*noopMeter) CreateObservableGauge(string, *MetricOptions) ObservableGauge {
	return noopObservableGaugeMetricInstance
}
func (*noopMeter) CreateObservableCounter(string, *MetricOptions) ObservableCounter {
	return noopObservableCounterMetricInstance
}
func (*noopMeter) CreateObservableUpDownCounter(string, *MetricOptions) ObservableUpDownCounter {
	return noopObservableUpDownCounterMetricInstance
}
func (*noopMeter) AddBatchObservableCallback(BatchObservableCallback, []Observable)    {}
func (*noopMeter) RemoveBatchObservableCallback(BatchObservableCallback, []Observable) {}

// CreateNoopMeter returns the no-op meter (always the same one).
func CreateNoopMeter() Meter { return noopMeterInstance }

// noopMeterProvider is NoopMeterProvider: every meter is the no-op meter.
type noopMeterProvider struct{}

func (*noopMeterProvider) GetMeter(string, string, *MeterOptions) Meter { return noopMeterInstance }

var noopMeterProviderInstance MeterProvider = &noopMeterProvider{}

const metricsAPIName = "metrics"

// MetricsAPI is the metrics API (JavaScript's MetricsAPI class; its instance is Metrics).
type MetricsAPI struct{}

// SetGlobalMeterProvider registers the meter provider and reports whether it was registered.
func (m *MetricsAPI) SetGlobalMeterProvider(provider MeterProvider) bool {
	return registerGlobal(metricsAPIName, provider, Diag, false)
}

// GetMeterProvider returns the global meter provider, or the no-op provider.
func (m *MetricsAPI) GetMeterProvider() MeterProvider {
	if p, ok := getGlobal(metricsAPIName).(MeterProvider); ok && !jsvalue.IsNull(p) {
		return p
	}
	return noopMeterProviderInstance
}

// GetMeter returns a meter of the global meter provider (an empty version is undefined).
func (m *MetricsAPI) GetMeter(name, version string, options *MeterOptions) Meter {
	return m.GetMeterProvider().GetMeter(name, version, options)
}

// Disable removes the global meter provider.
func (m *MetricsAPI) Disable() { unregisterGlobal(metricsAPIName, Diag) }
