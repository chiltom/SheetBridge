package main

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/chiltom/SheetBridge/internal/apperrors"
)

// responseWriterDelegator implements some of the more boilerplate tasks for a response writer
type responseWriterDelegator struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// WriteHeader writes an HTTP status code to the response headers and notes its completion
func (rwd *responseWriterDelegator) WriteHeader(statusCode int) {
	if rwd.wroteHeader {
		return
	}
	rwd.status = statusCode
	rwd.ResponseWriter.WriteHeader(statusCode)
	rwd.wroteHeader = true
}

// Write ensures that a status code was written to the response headers
// and then writes the byte string using the http.ResponseWriter
func (rwd *responseWriterDelegator) Write(b []byte) (int, error) {
	if !rwd.wroteHeader {
		rwd.WriteHeader(http.StatusOK)
	}
	return rwd.ResponseWriter.Write(b)
}

// logRequest attaches an extra function to an http.Handler to ensure
// detailed request logging
func (app *application) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		delegator := &responseWriterDelegator{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(delegator, r)
		app.logger.Infof("%s - %s %s %s (status %d) (%s)",
			r.RemoteAddr, r.Proto, r.Method, r.URL.RequestURI(), delegator.status, time.Since(start))
	})
}

// recoverPanic provides graceful error handling when the server is killing processes
func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				app.logger.Errorf("Panic recovered: %s\n%s", err, debug.Stack())
				app.serverError(w, r, apperrors.Wrap(fmt.Errorf("%v", err), apperrors.ErrInternalServer, "A critical error occurred"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
