package main

import (
    "fmt"
	"ica-caldav/ica"
	"net/http"
)

func newServerForSetup(ica *ica.ICA) http.Handler {
    return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(rw, "Hello, world")
    })
}

