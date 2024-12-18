package main

import (
    "bytes"
    "io"
    "time"
	"ica-caldav/ica"
    "net/http"
	"log"
	"log/slog"
	"os"

	"github.com/emersion/go-webdav/caldav"
)


func main() {
    sessionId := os.Getenv("SESSION_ID")
    ica := ica.New(sessionId)
    backend := NewIcaBackend(&ica)
    handler := caldav.Handler{Backend: backend}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	slog.Info("Starting")

    log.Fatal(http.ListenAndServe(":8080", WithLogging(&handler)))
}

func WithLogging(h http.Handler) http.Handler {
	logFn := func(rw http.ResponseWriter, r *http.Request) {
		lrw := LoggingResponseWriter{rw, ""}
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewBuffer(body))
		start := time.Now()

		h.ServeHTTP(&lrw, r) // serve the original request

		timeElapsed := time.Since(start)
		// log request details
		slog.Info("Request",
			"method", r.Method,
			"uri", r.RequestURI,
			"duration", timeElapsed,
		)
	}
	return http.HandlerFunc(logFn)
}

type LoggingResponseWriter struct {
	http.ResponseWriter
	body string
}

func (r *LoggingResponseWriter) Write(b []byte) (int, error) {
	r.body += string(b)
	return r.ResponseWriter.Write(b)
}
