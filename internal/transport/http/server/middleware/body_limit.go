package middleware

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

var ErrRequestBodyTooLarge = errors.New("analytics request body exceeds configured limit")

const ctxBodyLimitReaderKey = "request_body_limit_reader"

// RequestBodyLimitExceeded reports actual overflow recorded by the limiting reader.
// Keep the reader in the context because request validation may replace Request.Body.
func RequestBodyLimitExceeded(c *gin.Context) bool {
	value, ok := c.Get(ctxBodyLimitReaderKey)
	if !ok {
		return false
	}
	reader, ok := value.(*limitedReadCloser)
	return ok && reader.overflow
}

func AnalyticsBodyLimit(maxBytes int64) gin.HandlerFunc {
	return RequestBodyLimit("/v1/analytics/events/batch", maxBytes)
}

func RequestBodyLimit(path string, maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 || c.Request.URL.Path != path {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			writeRequestBodyTooLarge(c, maxBytes)
			return
		}
		reader := &limitedReadCloser{
			source:    c.Request.Body,
			remaining: maxBytes + 1,
		}
		c.Set(ctxBodyLimitReaderKey, reader)
		c.Request.Body = reader
		c.Next()
	}
}

type limitedReadCloser struct {
	source    io.ReadCloser
	remaining int64
	overflow  bool
	reported  bool
}

func (r *limitedReadCloser) Read(p []byte) (int, error) {
	if r.reported {
		return 0, ErrRequestBodyTooLarge
	}
	if r.overflow {
		r.reported = true
		return 0, ErrRequestBodyTooLarge
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}

	n, err := r.source.Read(p)
	r.remaining -= int64(n)
	if r.remaining == 0 {
		r.overflow = true
		if errors.Is(err, io.EOF) {
			err = nil
		}
	}
	return n, err
}

func (r *limitedReadCloser) Close() error {
	return r.source.Close()
}

func writeRequestBodyTooLarge(c *gin.Context, maxBytes int64) {
	payload := gin.H{
		"code":       "validation",
		"reason":     "validation.failed",
		"message":    "validation failed",
		"request_id": GetRequestID(c),
		"details": []gin.H{{
			"field":  "body",
			"rule":   "max",
			"reason": fmt.Sprintf("must be at most %d bytes", maxBytes),
		}},
	}
	c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, payload)
}
