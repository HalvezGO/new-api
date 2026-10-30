package controller

import (
	"context"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
)

// When the client hangs up, retrying on another channel cannot deliver the
// response anywhere; it only burns upstream tokens for a dead request.
func TestDecideRelayRetryStopsWhenClientIsGone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := newRetryIntervalTestContext(t, ctx)
	cancel()

	clientGoneErr := types.NewErrorWithStatusCode(
		context.Canceled,
		types.ErrorCodeDoRequestFailed,
		http.StatusInternalServerError,
	)

	assert.NotEqual(t, "retry", decideRelayRetry(c, clientGoneErr, 3).Action,
		"a canceled client request must not be retried on another channel")
}

// The upstream call is canceled with the client, so the resulting error carries
// context.Canceled even though the status code looks retryable.
func TestDecideRelayRetryStopsOnCanceledUpstreamError(t *testing.T) {
	c := newRetryIntervalTestContext(t, nil)
	err := types.NewErrorWithStatusCode(
		context.Canceled,
		types.ErrorCodeDoRequestFailed,
		http.StatusBadGateway,
	)

	assert.NotEqual(t, "retry", decideRelayRetry(c, err, 3).Action)
}

// An ordinary upstream failure must still be retried.
func TestDecideRelayRetryStillRetriesRealUpstreamFailures(t *testing.T) {
	c := newRetryIntervalTestContext(t, nil)
	err := types.NewErrorWithStatusCode(
		assert.AnError,
		types.ErrorCodeBadResponseStatusCode,
		http.StatusTooManyRequests,
	)

	assert.Equal(t, "retry", decideRelayRetry(c, err, 3).Action)
}

func TestIsClientGoneDetectsRequestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := newRetryIntervalTestContext(t, ctx)
	cancel()

	assert.True(t, isClientGone(c, nil))
	assert.False(t, isClientGone(newRetryIntervalTestContext(t, nil), nil))
}

// A provider error that was already in flight when the client hung up must be
// settled as client-gone: otherwise the row blames the provider for a request
// whose caller had left (retry waits and the last attempt only see the client
// before they start).
func TestResolveClientGoneAtSettlement(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	canceledClient := newRetryIntervalTestContext(t, canceledCtx)

	upstreamErr := types.NewErrorWithStatusCode(
		assert.AnError,
		types.ErrorCodeBadResponseStatusCode,
		http.StatusTooManyRequests,
	)
	violationErr := types.NewErrorWithStatusCode(
		assert.AnError,
		types.ErrorCode("violation_fee.test"),
		http.StatusForbidden,
	)
	clientGoneErr := types.NewClientGoneError(context.Canceled)

	t.Run("no error stays nil", func(t *testing.T) {
		assert.Nil(t, resolveClientGoneAtSettlement(canceledClient, nil))
	})

	t.Run("live client keeps the provider error", func(t *testing.T) {
		got := resolveClientGoneAtSettlement(newRetryIntervalTestContext(t, nil), upstreamErr)

		assert.Same(t, upstreamErr, got)
	})

	t.Run("provider error after the client left becomes client-gone", func(t *testing.T) {
		got := resolveClientGoneAtSettlement(canceledClient, upstreamErr)

		assert.NotSame(t, upstreamErr, got)
		assert.True(t, types.IsClientGone(got))
		assert.Equal(t, types.StatusClientClosedRequest, got.StatusCode)
	})

	t.Run("violation fee keeps its own outcome", func(t *testing.T) {
		got := resolveClientGoneAtSettlement(canceledClient, violationErr)

		assert.Same(t, violationErr, got)
		assert.Equal(t, types.ErrorCode("violation_fee.test"), got.GetErrorCode())
	})

	t.Run("existing client-gone error is kept", func(t *testing.T) {
		got := resolveClientGoneAtSettlement(canceledClient, clientGoneErr)

		assert.Same(t, clientGoneErr, got)
	})
}
