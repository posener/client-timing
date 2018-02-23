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
type Option func(*Client)

// OptTransport sets the inner Client for the request
func OptTransport(inner http.RoundTripper) Option {
	return func(t *Client) {
		t.inner = inner
	}
}

// OptMetric sets the metric function which defines the metric name from the request
func OptMetric(metric func(*http.Request) string) Option {
	return func(t *Client) {
		t.metric = metric
	}
}

func OptName(name string) Option {
	return func(t *Client) {
		t.name = name
	}
}

// OptDesc sets the desc function which defines the metric description from the request
func OptDesc(desc func(*http.Request) string) Option {
	return func(t *Client) {
		t.desc = desc
	}
}

// OptUpdate sets the update function which updates the metric according to response and error
// received from completing the round trip
func OptUpdate(update func(*servertiming.Metric, *http.Response, error)) Option {
	return func(t *Client) {
		t.update = update
	}
}

// New returns a instrumented constructor for http client and transport.
func New(opts ...Option) *Client {
	// create default round tripper
	t := &Client{
		inner:  http.DefaultTransport,
		metric: defaultMetric,
		desc:   defaultDesc,
		update: defaultUpdate,
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

	if t.name != "" {
		metric.Extra["source"] = t.name
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
	if err != nil {
		m.Extra["error"] = err.Error()
	} else {
		m.Extra["code"] = strconv.Itoa(resp.StatusCode)
	}
}
