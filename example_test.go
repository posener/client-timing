package clienttiming_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"regexp"

	"strings"

	"github.com/mitchellh/go-server-timing"
	"github.com/posener/client-timing"
)

func ExampleClient() {

	s1 := httptest.NewServer(servertiming.Middleware(
		&handler{
			client: clienttiming.New(clienttiming.OptName("server1")),
			name:   "server1",
		},
		nil,
	))
	defer s1.Close()

	s2 := httptest.NewServer(servertiming.Middleware(
		&handler{
			name:   "server2",
			client: clienttiming.New(clienttiming.OptName("server2")),
			addr:   []string{s1.URL + "/level2"},
		},
		nil,
	))
	defer s2.Close()

	h := servertiming.Middleware(
		&handler{
			name:   "handler",
			client: clienttiming.New(clienttiming.OptName("handler")),
			addr: []string{
				s1.URL + "/level1",
				s2.URL + "/level1",
			},
		},
		nil,
	)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/level0", nil))

	headers := rec.Header().Get(servertiming.HeaderKey)

	// sanitize the header from changing phrases
	headers = strings.Replace(headers, serverName(s2), "to-server2", -1)
	headers = strings.Replace(headers, serverName(s1), "to-server1", -1)
	headers = dur.ReplaceAllString(headers, "dur=100")

	fmt.Printf("Timing headers: %s", headers)
	// Output:
	// Timing headers: to-server1;desc="GET /level2";dur=100;source="server2";code=200,to-server1;desc="GET /level1";dur=100;source="handler";code=200,to-server2;desc="GET /level1";dur=100;source="handler";code=200
}

var dur = regexp.MustCompile(`dur=\d+\.?\d*`)

type handler struct {
	name string
	// client is used by te handler to send http requests
	client *clienttiming.Client
	// requests defines addresses for upstream GET requests
	addr []string
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := h.client.Client(r.Context())

	// sleep to have some duration in headers
	time.Sleep(time.Millisecond * 50)

	// send request to all addresses
	for _, addr := range h.addr {
		resp, err := c.Get(addr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp.Body.Close()
	}

	// write headers
	w.WriteHeader(http.StatusOK)
}

func serverName(s *httptest.Server) string {
	return strings.Replace(s.Listener.Addr().String(), ":", ".", -1)
}
