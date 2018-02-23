package clienttiming

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mitchellh/go-server-timing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	t.Parallel()

	s1 := httptest.NewServer(servertiming.Middleware(
		&handler{
			client: New(OptName("server1")),
			name:   "server1",
		},
		nil,
	))
	defer s1.Close()

	s2 := httptest.NewServer(servertiming.Middleware(
		&handler{
			name:   "server2",
			client: New(OptName("server2")),
			addr1:  s1.URL + "/level2",
		},
		nil,
	))
	defer s2.Close()

	h := servertiming.Middleware(
		&handler{
			name:   "handler",
			client: New(OptName("handler")),
			addr1:  s1.URL + "/level1",
			addr2:  s2.URL + "/level1",
		},
		nil,
	)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/level0", nil))

	timings, err := servertiming.ParseHeader(rec.Header().Get(servertiming.HeaderKey))
	require.Nil(t, err)

	clearDurations(t, timings.Metrics)

	assert.Equal(
		t,
		[]*servertiming.Metric{
			{
				Name: serverName(s1),
				Desc: "GET /level2",
				Extra: map[string]string{
					"code":   "200",
					"source": "server2",
				},
			},
			{
				Name: serverName(s1),
				Desc: "GET /level1",
				Extra: map[string]string{
					"code":   "200",
					"source": "handler",
				},
			},
			{
				Name: serverName(s2),
				Desc: "GET /level1",
				Extra: map[string]string{
					"code":   "200",
					"source": "handler",
				},
			},
		},
		timings.Metrics,
	)
}

type handler struct {
	addr1, addr2 string
	name         string
	client       *Client
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

func clearDurations(t *testing.T, metrics []*servertiming.Metric) {
	t.Helper()
	for _, m := range metrics {
		assert.True(t, m.Duration > 0)
		m.Duration = 0
	}
}
