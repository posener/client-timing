package clienttiming

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/mitchellh/go-server-timing"
)

// Option is client-timing transport option function
type Option func(*Transport)

// OptTransport sets the inner Transport for the request
func OptTransport(inner http.RoundTripper) Option {
	return func(t *Transport) {
		t.inner = inner
	}
}

// OptMetric sets the metric function which defines the metric name from the request
func OptMetric(metric func(*http.Request) string) Option {
	return func(t *Transport) {
		t.metric = metric
	}
}

// OptDesc sets the desc function which defines the metric description from the request
func OptDesc(desc func(*http.Request) string) Option {
	return func(t *Transport) {
		t.desc = desc
	}
}

// OptUpdate sets the update function which updates the metric according to response and error
// received from completing the round trip
func OptUpdate(update func(*servertiming.Metric, *http.Response, error)) Option {
	return func(t *Transport) {
		t.update = update
	}
}

// NewTransport returns a new instrumented Transport for server-timing
func NewTransport(ctx context.Context, opts ...Option) http.RoundTripper {
	// create default round tripper
	t := &Transport{
		inner:  http.DefaultTransport,
		metric: defaultMetric,
		desc:   defaultDesc,
		update: defaultUpdate,
		timing: servertiming.FromContext(ctx),
	}

	// apply options
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// NewClient returns a new http client instrumented for server-timing
func NewClient(ctx context.Context, opts ...Option) *http.Client {
	return &http.Client{Transport: NewTransport(ctx, opts...)}
}

// Transport is instrumented http Transport
type Transport struct {
	// inner is the inner Transport used for sending the request and receiving the response
	inner http.RoundTripper
	// timing is the timing header
	timing *servertiming.Header
	// metric is a function that sets the metric name from a given request
	// desc is a function that sets the metric description from a given request
	metric, desc func(*http.Request) string
	// update updates the metric data from the response and error received after
	// completing the roundtrip
	update func(*servertiming.Metric, *http.Response, error)
}

// RoundTrip implements the http.RoundTripper interface
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {

	// Start the metrics for the get
	metric := t.timing.NewMetric(t.metric(req)).WithDesc(t.desc(req)).Start()

	// Run the inner round trip
	resp, err := t.inner.RoundTrip(req)

	// Stop the metric after get
	metric.Stop()

	// update the metric with the response and error of the request
	t.update(metric, resp, err)

	// In case of round trip error, return it
	if err != nil {
		return nil, err
	}

	// Insert the timing headers from the response to the current headers
	// They should be inserted in the beginning since they happened before the get itself
	if responseHeaders, err := servertiming.ParseHeader(resp.Header.Get(servertiming.HeaderKey)); err == nil {
		t.timing.Metrics = append(responseHeaders.Metrics, t.timing.Metrics...)
	}

	return resp, err
}

// defaultMetric set the metric name as the request host
func defaultMetric(req *http.Request) string {
	return strings.Replace(req.Host, ":", ".", -1)
}

// defaultDesc set the metric description as the request method and path
func defaultDesc(req *http.Request) string {
	return fmt.Sprintf("%s %s", req.Method, req.URL.Path)
}

// defaultUpdate sets status code in metric if there was no error, otherwise it sets the error text.
func defaultUpdate(m *servertiming.Metric, resp *http.Response, err error) {
	if m.Extra == nil {
		m.Extra = make(map[string]string)
	}
	if err != nil {
		m.Extra["error"] = err.Error()
	} else {
		m.Extra["code"] = strconv.Itoa(resp.StatusCode)
	}
}
