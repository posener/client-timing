package clienttiming

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mitchellh/go-server-timing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	t.Parallel()

	s1 := httptest.NewServer(servertiming.Middleware(
		&handler{
			client: New(WithName("server1")),
			name:   "server1",
		},
		nil,
	))
	defer s1.Close()

	s2 := httptest.NewServer(servertiming.Middleware(
		&handler{
			name:     "server2",
			client:   New(WithName("server2")),
			requests: []string{s1.URL + "/level2"},
		},
		nil,
	))
	defer s2.Close()

	h := servertiming.Middleware(
		&handler{
			name:   "handler",
			client: New(WithName("handler")),
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
	// client is used by te handler to send http requests
	client *Client
	// requests defines addresses for upstream GET requests
	requests []string
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := h.client.Client(r.Context())

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

func TestOptions(t *testing.T) {
	t.Parallel()

	reqMetrics := []*servertiming.Metric{
		{Name: "golang.org", Desc: "GET ", Extra: map[string]string{"code": "200"}},
	}

	upstreamHeader := servertiming.Header{
		Metrics: []*servertiming.Metric{
			{Name: "github.com", Desc: "POST /api", Extra: map[string]string{"status": "201"}},
			{Name: "github.com", Desc: "GET /api/v2", Extra: map[string]string{"status": "404"}},
		},
	}

	tests := []struct {
		name string

		// options to test
		opts []Option

		// inner transport behaviour
		trResp *http.Response
		trErr  error

		// expected results
		wantErr     bool
		wantMetrics []*servertiming.Metric
	}{
		{
			name:        "no options",
			trResp:      &http.Response{StatusCode: http.StatusOK},
			wantMetrics: reqMetrics,
		},
		{
			name: "get upstream timing headers",
			trResp: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					servertiming.HeaderKey: []string{upstreamHeader.String()},
				},
			},
			wantMetrics: append(upstreamHeader.Metrics, reqMetrics...),
		},
		{
			name:    "error from upstream",
			trErr:   fmt.Errorf("failed"),
			wantErr: true,
			wantMetrics: []*servertiming.Metric{
				{Name: "golang.org", Desc: "GET ", Extra: map[string]string{"error": "failed"}},
			},
		},
		{
			name:   "with name",
			trResp: &http.Response{StatusCode: http.StatusOK},
			opts:   []Option{WithName("name")},
			wantMetrics: []*servertiming.Metric{
				{Name: "golang.org", Desc: "GET ", Extra: map[string]string{"code": "200", "source": "name"}},
			},
		},
		{
			name:   "with metric func",
			trResp: &http.Response{StatusCode: http.StatusOK},
			opts:   []Option{WithMetric(func(*http.Request) string { return "surprise" })},
			wantMetrics: []*servertiming.Metric{
				{Name: "surprise", Desc: "GET ", Extra: map[string]string{"code": "200"}},
			},
		},
		{
			name:   "with desc func",
			trResp: &http.Response{StatusCode: http.StatusOK},
			opts:   []Option{WithDesc(func(*http.Request) string { return "surprise" })},
			wantMetrics: []*servertiming.Metric{
				{Name: "golang.org", Desc: "surprise", Extra: map[string]string{"code": "200"}},
			},
		},
		{
			name:   "with update func",
			trResp: &http.Response{StatusCode: http.StatusOK},
			opts: []Option{WithUpdate(func(m *servertiming.Metric, resp *http.Response, err error) {
				m.Extra["big"] = "surprise"
				DefaultUpdate(m, resp, err)
			})},
			wantMetrics: []*servertiming.Metric{
				{Name: "golang.org", Desc: "GET ", Extra: map[string]string{"code": "200", "big": "surprise"}},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				tr     = new(mockTransport)
				opts   = append(tt.opts, WithTransport(tr))
				timing = New(opts...)
				header servertiming.Header
				req, _ = http.NewRequest(http.MethodGet, "https://golang.org", nil)
				client = timing.Client(servertiming.NewContext(req.Context(), &header))
			)

			// prepare the mocked round tripper for the test
			tr.On("RoundTrip", req).Return(tt.trResp, tt.trErr).Once()

			// run the request
			_, err := client.Do(req)
			assert.Equal(t, tt.wantErr, err != nil)

			// clear the timings because they change between tests
			clearTimes(t, header.Metrics)

			// assert metrics from response header are as expected
			assert.Equal(t, tt.wantMetrics, header.Metrics)

			// assert that the mock was called as expected
			tr.AssertExpectations(t)
		})
	}

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

type mockTransport struct {
	mock.Mock
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if err := args.Error(1); err != nil {
		return nil, err
	}
	return args.Get(0).(*http.Response), nil
}
