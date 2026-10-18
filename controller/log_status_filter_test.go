package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestParseLogStatusQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newContext := func(query string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/api/log?"+query, nil)
		return c
	}

	// An absent or malformed value must mean "no filter" (-1); only 0 would hide
	// every in-progress row from the default "all" view.
	assert.Equal(t, -1, parseLogStatusQuery(newContext("")))
	assert.Equal(t, -1, parseLogStatusQuery(newContext("status=abc")))
	assert.Equal(t, -1, parseLogStatusQuery(newContext("status=-1")))
	assert.Equal(t, 1, parseLogStatusQuery(newContext("status=1")))
	assert.Equal(t, 2, parseLogStatusQuery(newContext("status=2")))
}
