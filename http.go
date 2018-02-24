package clienttiming

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/mitchellh/go-server-timing"
)

// KeySource is the key in the metric in which the source name will be stored
const KeySource = "source"

// Transport returns a server-timing instrumented round tripper for the current context
func (t *Timer) Transport(ctx context.Context, opts ...Option) http.RoundTripper {
	tr := &transport{
		Timer:  *t,
		timing: servertiming.FromContext(ctx),
	}

	// apply extra options on the timer copy
	for _, opt := range opts {
		opt(&tr.Timer)
	}

	return tr
}

// Client returns a server-timing instrumented http timer for the current context
func (t *Timer) Client(ctx context.Context, opts ...Option) *http.Client {
	return &http.Client{Transport: t.Transport(ctx, opts...)}
}

type transport struct {
	Timer
	// timing is the timing header
	timing *servertiming.Header
}

// RoundTrip implements the http.RoundTripper interface
func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {

	// Start the metrics for the get
	metric := t.timing.NewMetric(t.metric(req)).WithDesc(t.desc(req))

	if metric.Extra == nil {
		metric.Extra = make(map[string]string)
	}

	// add the timer name as a source to the metric
	if t.name != "" {
		metric.Extra[KeySource] = t.name
	}
	metric.Start()

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
	InsertMetrics(t.timing, resp.Header)

	return resp, err
}

// InsertMetrics inserts to servertiming header metrics from an HTTP header of another request
// They are prepended since they happened before the metrics of the header itself
func InsertMetrics(h *servertiming.Header, headers http.Header) {
	more, err := servertiming.ParseHeader(headers.Get(servertiming.HeaderKey))
	if err != nil {
		return
	}
	h.Metrics = append(more.Metrics, h.Metrics...)
}

// DefaultMetric set the metric name as the request host
func DefaultMetric(req *http.Request) string {
	return strings.Replace(req.Host, ":", ".", -1)
}

// DefaultDesc set the metric description as the request method and path
func DefaultDesc(req *http.Request) string {
	return fmt.Sprintf("%s %s", req.Method, req.URL.Path)
}

// DefaultUpdate sets status code in metric if there was no error, otherwise it sets the error text.
func DefaultUpdate(m *servertiming.Metric, resp *http.Response, err error) {
	if err != nil {
		m.Extra["error"] = err.Error()
	} else {
		m.Extra["code"] = strconv.Itoa(resp.StatusCode)
	}
}
