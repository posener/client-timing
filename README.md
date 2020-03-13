# clienttiming

Package clienttiming gives an HTTP client for go-server-timing middleware.

It provides:

An HTTP `Client` or `RoundTripper`, fully compatible with Go's standard library.
Automatically time HTTP requests sent from an HTTP handler.
Collects all timing headers from upstream servers.
Customize timing headers according to the request, response and error of the HTTP round trip.

## Sub Packages

* [example](./example)


---

Created by [goreadme](https://github.com/apps/goreadme)
