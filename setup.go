package main

import (
	"ica-caldav/ica"
	"io"
	"net/http"
	"text/template"
)

func newServerForSetup(authenticator *ica.BankIDAuthenticator) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		// TODO: Check if we actually need setup first
		executeTemplate(rw, "index.html", nil)
	})

	mux.HandleFunc("/start", func(rw http.ResponseWriter, r *http.Request) {
		err := authenticator.Start()
		if err != nil {
			executeTemplate(rw, "error.html", err)
			return
		}

		isFinished, qrCode, err := authenticator.State()
		if err != nil {
			executeTemplate(rw, "error.html", err)
		} else if isFinished {
			executeTemplate(rw, "finished.html", nil)
		} else {
			executeTemplate(rw, "bank-id.html", qrCode)
		}
	})

	mux.HandleFunc("/status", func(rw http.ResponseWriter, r *http.Request) {
		isFinished, qrCode, err := authenticator.State()
		if err != nil {
			executeTemplate(rw, "error.html", err)
		} else if isFinished {
			executeTemplate(rw, "finished.html", nil)
		} else {
			executeTemplate(rw, "bank-id.html", qrCode)
		}
	})

	return mux
}

// It's probably quicker to just fetch templates once, but this makes it support live-reload.
func executeTemplate(rw io.Writer, name string, val any) {
	template, err := template.ParseGlob("templates/*")
	if err != nil {
		panic(err)
	}
	template.ExecuteTemplate(rw, name, val)
}
