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
func TestShouldRetryStopsWhenClientIsGone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := newRetryIntervalTestContext(t, ctx)
	cancel()

	clientGoneErr := types.NewErrorWithStatusCode(
		context.Canceled,
		types.ErrorCodeDoRequestFailed,
		http.StatusInternalServerError,
	)

	assert.False(t, shouldRetry(c, clientGoneErr, 3),
		"a canceled client request must not be retried on another channel")
}

// The upstream call is canceled with the client, so the resulting error carries
// context.Canceled even though the status code looks retryable.
func TestShouldRetryStopsOnCanceledUpstreamError(t *testing.T) {
	c := newRetryIntervalTestContext(t, nil)
	err := types.NewErrorWithStatusCode(
		context.Canceled,
		types.ErrorCodeDoRequestFailed,
		http.StatusBadGateway,
	)

	assert.False(t, shouldRetry(c, err, 3))
}

// An ordinary upstream failure must still be retried.
func TestShouldRetryStillRetriesRealUpstreamFailures(t *testing.T) {
	c := newRetryIntervalTestContext(t, nil)
	err := types.NewErrorWithStatusCode(
		assert.AnError,
		types.ErrorCodeBadResponseStatusCode,
		http.StatusTooManyRequests,
	)

	assert.True(t, shouldRetry(c, err, 3))
}

func TestIsClientGoneDetectsRequestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := newRetryIntervalTestContext(t, ctx)
	cancel()

	assert.True(t, isClientGone(c, nil))
	assert.False(t, isClientGone(newRetryIntervalTestContext(t, nil), nil))
}
