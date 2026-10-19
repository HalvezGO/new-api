package types

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsClientGone(t *testing.T) {
	tests := []struct {
		name string
		err  *NewAPIError
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{
			name: "context canceled",
			err:  NewErrorWithStatusCode(context.Canceled, ErrorCodeDoRequestFailed, http.StatusInternalServerError),
			want: true,
		},
		{
			name: "wrapped context canceled",
			err:  NewErrorWithStatusCode(fmt.Errorf("do request failed: %w", context.Canceled), ErrorCodeDoRequestFailed, http.StatusInternalServerError),
			want: true,
		},
		{
			name: "client gone error",
			err:  NewClientGoneError(context.Canceled),
			want: true,
		},
		{
			name: "upstream timeout",
			err:  NewErrorWithStatusCode(context.DeadlineExceeded, ErrorCodeDoRequestFailed, http.StatusInternalServerError),
			want: false,
		},
		{
			name: "ordinary upstream failure",
			err:  NewErrorWithStatusCode(errors.New("upstream returned 429"), ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsClientGone(tt.err))
		})
	}
}

// A client-gone error must not be retried and must not be filed as a channel
// error log; otherwise a user hanging up would look like a provider outage.
func TestNewClientGoneErrorIsNotRetriedOrLogged(t *testing.T) {
	err := NewClientGoneError(context.Canceled)

	assert.True(t, IsSkipRetryError(err))
	assert.False(t, IsRecordErrorLog(err))
	assert.Equal(t, StatusClientClosedRequest, err.StatusCode)
	assert.Equal(t, ErrorCodeClientGone, err.GetErrorCode())
}
