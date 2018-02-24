package clienttiming

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/mitchellh/go-server-timing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHTTP(t *testing.T) {
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

		// opts are options for the new timer
		opts []Option

		// clientOpts are options that are applied when client is created
		clientOpts []Option

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
		{
			name:       "client options override timer options",
			trResp:     &http.Response{StatusCode: http.StatusOK},
			opts:       []Option{WithName("name")},
			clientOpts: []Option{WithName("other-name")},
			wantMetrics: []*servertiming.Metric{
				{Name: "golang.org", Desc: "GET ", Extra: map[string]string{"code": "200", "source": "other-name"}},
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
				client = timing.Client(servertiming.NewContext(req.Context(), &header), tt.clientOpts...)
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
