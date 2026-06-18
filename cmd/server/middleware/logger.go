package middleware

import (
	"net/http"
	"time"

	"ragsume-core/logger"

	"github.com/go-chi/chi/v5/middleware"
)

// RequestLogger logs basic information about each HTTP request.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"request_id", middleware.GetReqID(r.Context()),
			"status", ww.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
		)
	})
}
