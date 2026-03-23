package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingFlushWriter struct {
	gin.ResponseWriter
	flushCount int
}

func (w *countingFlushWriter) Flush() {
	w.flushCount++
	w.ResponseWriter.Flush()
}

func newCountingFlushContext() (*gin.Context, *countingFlushWriter, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/stream", nil)
	writer := &countingFlushWriter{ResponseWriter: c.Writer}
	c.Writer = writer
	return c, writer, recorder
}

func TestStringDataFlushThrottledUntilDone(t *testing.T) {
	t.Parallel()

	c, writer, recorder := newCountingFlushContext()

	require.NoError(t, StringData(c, "chunk-1"))
	assert.Equal(t, 0, writer.flushCount)

	Done(c)

	assert.Equal(t, 1, writer.flushCount)
	assert.Contains(t, recorder.Body.String(), "data: chunk-1")
	assert.Contains(t, recorder.Body.String(), "data: [DONE]")
}

func TestPingDataAlwaysFlushes(t *testing.T) {
	t.Parallel()

	c, writer, recorder := newCountingFlushContext()

	require.NoError(t, StringData(c, "chunk-1"))
	require.NoError(t, PingData(c))

	assert.Equal(t, 1, writer.flushCount)
	assert.Contains(t, recorder.Body.String(), ": PING")
}

func TestStringDataFlushesOnTimeThreshold(t *testing.T) {
	t.Parallel()

	c, writer, recorder := newCountingFlushContext()

	require.NoError(t, StringData(c, "chunk-1"))
	state := getStreamFlushState(c)
	require.NotNil(t, state)
	state.lastFlushTime = time.Now().Add(-streamFlushTimeThreshold - time.Millisecond)

	require.NoError(t, StringData(c, "chunk-2"))

	assert.Equal(t, 1, writer.flushCount)
	assert.True(t, strings.Contains(recorder.Body.String(), "data: chunk-2"))
}
