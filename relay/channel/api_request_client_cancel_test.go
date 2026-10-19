package channel

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubApiAdaptor implements just enough of Adaptor for DoApiRequest.
type stubApiAdaptor struct {
	Adaptor
	url string
}

func (s *stubApiAdaptor) GetRequestURL(*relaycommon.RelayInfo) (string, error) {
	return s.url, nil
}

func (s *stubApiAdaptor) SetupRequestHeader(*gin.Context, *http.Header, *relaycommon.RelayInfo) error {
	return nil
}

// A relay request builds its upstream request with http.NewRequest, which
// carries context.Background(). When the client hangs up, the upstream call
// must be aborted instead of running to completion: otherwise the proxy keeps
// consuming upstream tokens (and the retry loop keeps trying new channels) for
// a request nobody is waiting for.
func TestDoApiRequestAbortsWhenClientDisconnects(t *testing.T) {
	service.InitHttpClient()
	gin.SetMode(gin.TestMode)

	upstreamStarted := make(chan struct{})
	upstreamAborted := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(upstreamStarted)
		// Hold the response until the upstream request is canceled.
		select {
		case <-r.Context().Done():
			close(upstreamAborted)
		case <-time.After(10 * time.Second):
		}
	}))
	defer upstream.Close()

	clientCtx, cancelClient := context.WithCancel(context.Background())
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/relay", nil).WithContext(clientCtx)

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	adaptor := &stubApiAdaptor{url: upstream.URL}

	var (
		doErr error
		done  = make(chan struct{})
	)
	go func() {
		defer close(done)
		resp, err := DoApiRequest(adaptor, ctx, info, nil)
		if resp != nil {
			_ = resp.Body.Close()
		}
		doErr = err
	}()

	<-upstreamStarted
	cancelClient()

	select {
	case <-upstreamAborted:
	case <-time.After(5 * time.Second):
		t.Fatal("upstream request was not canceled after the client disconnected")
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("DoApiRequest did not return after the client disconnected")
	}

	require.Error(t, doErr, "the upstream call must fail once the client is gone")
	var apiErr *types.NewAPIError
	require.True(t, errors.As(doErr, &apiErr), "expected a NewAPIError, got %T", doErr)
	assert.True(t, types.IsClientGone(apiErr),
		"an abort caused by the client hanging up must be classified as client-gone")
	assert.True(t, types.IsSkipRetryError(apiErr),
		"a client-gone error must not be retried on another channel")
}

// Task submissions are deliberately not tied to the client context: aborting
// one mid-flight can only orphan a task the upstream may already have created,
// while a completed submission stays visible to the caller in the task list.
func TestDoTaskApiRequestCompletesAfterClientDisconnects(t *testing.T) {
	service.InitHttpClient()
	gin.SetMode(gin.TestMode)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"task_1"}`))
	}))
	defer server.Close()

	clientCtx, cancelClient := context.WithCancel(context.Background())
	cancelClient() // the caller already hung up when the submission went out
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil).WithContext(clientCtx)

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	adaptor := &stubTaskAdaptor{baseURL: server.URL}

	resp, err := DoTaskApiRequest(adaptor, ctx, info, strings.NewReader(`{"model":"test-model"}`))
	require.NoError(t, err, "a task submission must not be canceled by a client disconnect")
	require.NotNil(t, resp)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
