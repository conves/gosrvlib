// Package metrics defines a general interface for metrics instrumentation.
package metrics

import (
	"database/sql"
	"net/http"
)

// Client is an interface type for the metrics functions.
type Client interface {
	// InstrumentDB wraps a sql.DB to collect metrics.
	InstrumentDB(dbName string, db *sql.DB) error

	// InstrumentHandler wraps a http.Handler to collect metrics.
	InstrumentHandler(path string, handler http.HandlerFunc) http.Handler

	// InstrumentRoundTripper is a middleware that wraps the provided http.RoundTripper to observe the request result with default metrics.
	InstrumentRoundTripper(next http.RoundTripper) http.RoundTripper

	// MetricsHandlerFunc returns an http handler function to serve the metrics endpoint.
	MetricsHandlerFunc() http.HandlerFunc

	// IncLogLevelCounter counts the number of errors for each log severity level.
	IncLogLevelCounter(level string)

	// IncErrorCounter increments the number of errors by task, operation and error code.
	IncErrorCounter(task, operation, code string)

	// Close method.
	Close() error
}

// Default is the default implementation for the Client interface.
type Default struct{}

// InstrumentDB wraps a sql.DB to collect metrics.
func (c *Default) InstrumentDB(dbName string, db *sql.DB) error {
	return nil
}

// InstrumentHandler returns the input handler.
func (c *Default) InstrumentHandler(path string, handler http.HandlerFunc) http.Handler {
	return handler
}

// InstrumentRoundTripper returns the input Roundtripper.
func (c *Default) InstrumentRoundTripper(next http.RoundTripper) http.RoundTripper {
	return next
}

// MetricsHandlerFunc returns an http handler function.
func (c *Default) MetricsHandlerFunc() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`OK`)) }
}

// IncLogLevelCounter is an empty function.
func (c *Default) IncLogLevelCounter(level string) {}

// IncErrorCounter is an empty function.
func (c *Default) IncErrorCounter(task, operation, code string) {}

// Close method.
func (c *Default) Close() error { return nil }
