package clienttiming

import (
	"net/http"

	"github.com/mitchellh/go-server-timing"
)

// Timer constructs instrumented clients for server-timing
type Timer struct {
	// inner is the inner Client used for sending the request and receiving the response
	inner http.RoundTripper
	// metric is a function that sets the metric name from a given request
	// desc is a function that sets the metric description from a given request
	metric, desc func(*http.Request) string
	// update updates the metric data from the response and error received after
	// completing the round trip
	update func(*servertiming.Metric, *http.Response, error)
	// name is the name of the service holding the timer
	// it will be added to the timing extra data as "source"
	name string
}

// Option is timer-timing mockTransport option function
type Option func(*Timer)

// New returns a instrumented constructor for http timer and mockTransport.
func New(opts ...Option) *Timer {
	// create default round tripper
	t := &Timer{
		inner:  http.DefaultTransport,
		metric: DefaultMetric,
		desc:   DefaultDesc,
		update: DefaultUpdate,
	}

	// apply options
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// WithName updates the source key in the metric to this name
// It is used to give a name for the timer
func WithName(name string) Option {
	return func(t *Timer) {
		t.name = name
	}
}

// WithTransport sets the inner Client for the request
func WithTransport(inner http.RoundTripper) Option {
	return func(t *Timer) {
		t.inner = inner
	}
}

// WithMetric sets the metric function which defines the metric name from the request
func WithMetric(metric func(*http.Request) string) Option {
	return func(t *Timer) {
		t.metric = metric
	}
}

// WithDesc sets the desc function which defines the metric description from the request
func WithDesc(desc func(*http.Request) string) Option {
	return func(t *Timer) {
		t.desc = desc
	}
}

// WithUpdate sets the update function which updates the metric according to response and error
// received from completing the round trip
func WithUpdate(update func(*servertiming.Metric, *http.Response, error)) Option {
	return func(t *Timer) {
		t.update = update
	}
}
