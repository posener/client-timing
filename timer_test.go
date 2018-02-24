package clienttiming

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mitchellh/go-server-timing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimer(t *testing.T) {
	t.Parallel()

	s1 := httptest.NewServer(servertiming.Middleware(
		&handler{
			timer: New(WithName("server1")),
			name:  "server1",
		},
		nil,
	))
	defer s1.Close()

	s2 := httptest.NewServer(servertiming.Middleware(
		&handler{
			name:     "server2",
			timer:    New(WithName("server2")),
			requests: []string{s1.URL + "/level2"},
		},
		nil,
	))
	defer s2.Close()

	h := servertiming.Middleware(
		&handler{
			name:  "handler",
			timer: New(WithName("handler")),
			requests: []string{
				s1.URL + "/level1",
				s2.URL + "/level1",
			},
		},
		nil,
	)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/level0", nil))

	timings, err := servertiming.ParseHeader(rec.Header().Get(servertiming.HeaderKey))
	require.Nil(t, err)

	clearTimes(t, timings.Metrics)

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
	name string
	// timer is used by te handler to send http requests
	timer *Timer
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
}

func serverName(s *httptest.Server) string {
	return strings.Replace(s.Listener.Addr().String(), ":", ".", -1)
}

func clearTimes(t *testing.T, metrics []*servertiming.Metric) {
	t.Helper()
	for i, m := range metrics {
		// remove Duration and startTime from metric
		metrics[i] = &servertiming.Metric{
			Name:  m.Name,
			Desc:  m.Desc,
			Extra: m.Extra,
		}
	}
}
