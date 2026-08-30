package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BodyLimit caps how much of a request body the server will read.
//
// Without this, a client can stream an unbounded body and the JSON decoder will
// happily buffer all of it — a cheap way to exhaust the process's memory.
// http.MaxBytesReader stops the read at the limit and makes the subsequent
// bind return a *http.MaxBytesError, which response.BindError turns into a 413.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes > 0 && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
