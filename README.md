# client-timing

[![Build Status](https://travis-ci.org/posener/client-timing.svg?branch=master)](https://travis-ci.org/posener/client-timing)
[![codecov](https://codecov.io/gh/posener/client-timing/branch/master/graph/badge.svg)](https://codecov.io/gh/posener/client-timing)
[![GoDoc](https://godoc.org/github.com/posener/client-timing?status.svg)](http://godoc.org/github.com/posener/client-timing)
[![Go Report Card](https://goreportcard.com/badge/github.com/posener/client-timing)](https://goreportcard.com/report/github.com/posener/client-timing)

An HTTP client for [go-server-timing](https://github.com/mitchellh/go-server-timing) middleware.

## Features:

* An HTTP `Client` or `RoundTripper`, fully compatible with Go's standard library.
* Automatically time HTTP requests sent from an HTTP handler.
* Collects all timing headers from upstream servers.
* Customize timing headers according to the request, response and error of the HTTP round trip.

## Install

`go get -u github.com/posener/client-timing`

## Usage

1. Add a `*clienttiming.Timer` to your server handler, or create it in the handler function itself.
2. Wrap the `http.Handler` with [`servertiming.Middleware`](https://godoc.org/github.com/mitchellh/go-server-timing#Middleware).
2. In the handler function, having `timer` of type `*clienttiming.Timer` and `req` is the `*http.Request`:

    a. Create an [`*http.Client`](https://godoc.org/net/http#Client) using `timer.Client(req.Context())`
    
    b. Or create an [`http.RoundTripper`](https://godoc.org/net/http#RoundTripper) using `timer.Transport(req.Context())`
    
3. Use option a or b directly or inject it to a library that accepts them, in your outgoing HTTP request
   from the handler.
4. That is it! the timing header will appear in the response from the handler.

```go
type handler struct {
	timer *clienttiming.Timer
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := h.timer.Client(r.Context())
	
	resp, err := c.Get("https://golang.org/")
	// handler resp, and error
}
```

See [`Timer` test](./timer_test.go).
