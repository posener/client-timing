package clienttiming_test

import (
	"fmt"
	"io/ioutil"
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
			addr1:  s1.URL + "/level2",
		},
		nil,
	))
	defer s2.Close()

	h := servertiming.Middleware(
		&handler{
			name:   "handler",
			client: clienttiming.New(clienttiming.OptName("handler")),
			addr1:  s1.URL + "/level1",
			addr2:  s2.URL + "/level1",
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
	addr1, addr2 string
	name         string
	client       *clienttiming.Client
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := h.client.Client(r.Context())
	time.Sleep(time.Millisecond * 50)
	b1, err := get(c, h.addr1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	b2, err := get(c, h.addr2)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// write headers
	w.WriteHeader(http.StatusOK)

	// write body
	if b1 != nil {
		w.Write(b1)
	}
	if b2 != nil {
		w.Write(b2)
	}
	w.Write([]byte(h.name + "\n"))
}

func get(c *http.Client, addr string) ([]byte, error) {
	if addr == "" {
		return nil, nil
	}
	resp, err := c.Get(addr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func serverName(s *httptest.Server) string {
	return strings.Replace(s.Listener.Addr().String(), ":", ".", -1)
}
