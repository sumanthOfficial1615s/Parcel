package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/gin-gonic/gin"
)

// respondError sends a JSON error to the client. For server errors (5xx)
// it also logs the underlying cause and the file:line of the caller,
// so a bare "500" in the access log always has a matching reason.
func respondError(c *gin.Context, status int, msg string, errs ...error) {
	if status >= http.StatusInternalServerError {
		_, file, line, _ := runtime.Caller(1)

		var cause error
		if len(errs) > 0 {
			cause = errs[0]
		}

		slog.Error(msg,
			"error", cause,
			"at", fmt.Sprintf("%s:%d", filepath.Base(file), line),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
		)
	}

	c.JSON(status, gin.H{"error": msg})
}
