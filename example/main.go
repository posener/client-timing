package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/mitchellh/go-server-timing"
	"github.com/posener/client-timing"
)

func main() {

	s1 := httptest.NewServer(servertiming.Middleware(
		&handler{
			timer: clienttiming.New(clienttiming.WithName("server1")),
			name:  "server1",
		},
		nil,
	))
	defer s1.Close()

	s2 := httptest.NewServer(servertiming.Middleware(
		&handler{
			name:     "server2",
			timer:    clienttiming.New(clienttiming.WithName("server2")),
			requests: []string{s1.URL + "/level2"},
		},
		nil,
	))
	defer s2.Close()

	h := servertiming.Middleware(
		&handler{
			name:  "handler",
			timer: clienttiming.New(clienttiming.WithName("handler")),
			requests: []string{
				s1.URL + "/level1",
				s2.URL + "/level1",
			},
		},
		nil,
	)

	fmt.Println("Open your browser on http://localhost:8080 to see server-timing")

	http.ListenAndServe(":8080", h)

}

type handler struct {
	name string
	// timer is used by te handler to send http requests
	timer *clienttiming.Timer
	// requests defines addresses for upstream GET requests
	requests []string
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := h.timer.Client(r.Context())

	// sleep to have some duration in headers
	time.Sleep(time.Millisecond * 50)

	// send request to all addresses
	for _, addr := range h.requests {
		resp, err := c.Get(addr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp.Body.Close()
	}

	// write headers
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`
<h1>Welcome to Server Timing Example</h1>
<lu>
<li>Open developer tools</li>
<li>Go to network tab</li>
<li>Refresh the page (F5)</li>
<li>Go to Timing tab in the network tab</li>
<li>Check out the server timing section</li>
</lu>
`))
}
