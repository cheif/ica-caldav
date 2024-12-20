package main

import (
	"bytes"
	"context"
	"ica-caldav/ica"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/emersion/go-webdav/caldav"
)

func main() {
	authenticator := ica.NewBankIDAuthentication()
	htmlHandler := newServerForSetup(&authenticator)
	caldavHandler := withListCache(
		&authenticator,
	)

	handler := mux(htmlHandler, caldavHandler)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	slog.Info("Starting")

	log.Fatal(http.ListenAndServe(":5000", withLogging(handler)))
}

func mux(htmlHandler http.Handler, caldavHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// Use caldav handler if it's a `/user` path, or a non-supported html method
		htmlMethods := []string{http.MethodGet, http.MethodPost}
		if strings.HasPrefix(r.URL.Path, "/user") || !slices.Contains(htmlMethods, r.Method) {
			caldavHandler.ServeHTTP(rw, r)
		} else {
			htmlHandler.ServeHTTP(rw, r)
		}
	})
}

func withListCache(provider ica.SessionProvider) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		session, err := provider.GetSession()
		if err != nil {
			// Handle error here, now we just timeout?
			return
		} else {
			backend := NewIcaBackend(session)
			handler := caldav.Handler{Backend: backend}
			// Send in a list-cache, for performance
			newContext := context.WithValue(r.Context(), "listCache", ListCache{ica: session})
			handler.ServeHTTP(rw, r.WithContext(newContext))
		}
	})
}

func withLogging(h http.Handler) http.Handler {
	logFn := func(rw http.ResponseWriter, r *http.Request) {
		lrw := LoggingResponseWriter{rw, ""}
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewBuffer(body))
		start := time.Now()
		h.ServeHTTP(&lrw, r)

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
