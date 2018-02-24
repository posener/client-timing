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

// Option is client-timing mockTransport option function
type Option func(*Client)

// WithName updates the source key in the metric to this name
// It is used to give a name for the client
func WithName(name string) Option {
	return func(t *Client) {
		t.name = name
	}
}

// WithTransport sets the inner Client for the request
func WithTransport(inner http.RoundTripper) Option {
	return func(t *Client) {
		t.inner = inner
	}
}

// WithMetric sets the metric function which defines the metric name from the request
func WithMetric(metric func(*http.Request) string) Option {
	return func(t *Client) {
		t.metric = metric
	}
}

// WithDesc sets the desc function which defines the metric description from the request
func WithDesc(desc func(*http.Request) string) Option {
	return func(t *Client) {
		t.desc = desc
	}
}

// WithUpdate sets the update function which updates the metric according to response and error
// received from completing the round trip
func WithUpdate(update func(*servertiming.Metric, *http.Response, error)) Option {
	return func(t *Client) {
		t.update = update
	}
}

// New returns a instrumented constructor for http client and mockTransport.
func New(opts ...Option) *Client {
	// create default round tripper
	t := &Client{
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

// Transport returns a server-timing instrumented round tripper for the current context
func (c *Client) Transport(ctx context.Context) http.RoundTripper {
	return &transport{
		Client: *c,
		timing: servertiming.FromContext(ctx),
	}
}

// Client returns a server-timing instrumented http client for the current context
func (c *Client) Client(ctx context.Context) *http.Client {
	return &http.Client{Transport: c.Transport(ctx)}
}

// Client is instrumented http Client
type Client struct {
	// inner is the inner Client used for sending the request and receiving the response
	inner http.RoundTripper
	// metric is a function that sets the metric name from a given request
	// desc is a function that sets the metric description from a given request
	metric, desc func(*http.Request) string
	// update updates the metric data from the response and error received after
	// completing the round trip
	update func(*servertiming.Metric, *http.Response, error)
	// name is the name of the service holding the client
	// it will be added to the timing extra data as "source"
	name string
}

type transport struct {
	Client
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

	// add the client name as a source to the metric
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
