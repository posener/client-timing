# client-timing

[![Build Status](https://travis-ci.org/posener/client-timing.svg?branch=master)](https://travis-ci.org/posener/client-timing)
[![codecov](https://codecov.io/gh/posener/client-timing/branch/master/graph/badge.svg)](https://codecov.io/gh/posener/client-timing)
[![GoDoc](https://godoc.org/github.com/posener/client-timing?status.svg)](http://godoc.org/github.com/posener/client-timing)
[![Go Report Card](https://goreportcard.com/badge/github.com/posener/client-timing)](https://goreportcard.com/report/github.com/posener/client-timing)

An HTTP client for [go-server-timing](https://github.com/mitchellh/go-server-timing) middleware.

The client automatically time HTTP requests sent from an HTTP handler, and adds them to the response headers.
Additionally, it collects all timing headers from upstream server, and add them to the the response timing headers.

The timing headers can be customized according to the request, response and error of the HTTP round trip.

## Install

go get -u github.com/posener/client-timing

## Usage

1. Add a `*clienttiming.Client` to your server handler, or create it in the handler function itself.
2. Wrap the `http.Handler` with `servertiming.Middleware`.
2. In the handler function, having c of type `*clienttiming.Client` and `r` is the `*http.Request`:
    a. Create an `*http.Client` using `c.Client(r.Context())`
    b. Or create an `http.RoundTripper` using `c.Transport(r.Context())`
3. Use option a or b directly or inject it to a library that accepts them, in your outgoing HTTP request
   from the handler.
4. That is it! the timing header will appear in the response from the handler.

```go
type handler struct {
	client       *clienttiming.Client
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := h.client.Client(r.Context())
	
	resp, err := c.Get("https://golang.org/")
	// handler resp, and error
}
```

See [client test](./client_test.go).
