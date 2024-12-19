package main

import (
	"ica-caldav/ica"
	"net/http"
	"text/template"
)

func newServerForSetup(icaSession *ica.ICA) http.Handler {
	mux := http.NewServeMux()

	authenticator := ica.NewBankIDAuthentication(icaSession)

	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		// TODO: Check if we actually need setup first

		template, err := template.ParseGlob("templates/*")
		if err != nil {
			panic(err)
		}
		template.ExecuteTemplate(rw, "index.html", nil)
	})

	mux.HandleFunc("/start", func(rw http.ResponseWriter, r *http.Request) {
		template, err := template.ParseGlob("templates/*")
		if err != nil {
			panic(err)
		}

		err = authenticator.Start()
		if err != nil {
			template.ExecuteTemplate(rw, "error.html", err)
			return
		}

		isFinished, qrCode, err := authenticator.State()
		if err != nil {
			template.ExecuteTemplate(rw, "error.html", err)
		} else if isFinished {
			template.ExecuteTemplate(rw, "finished.html", nil)
		} else {
			template.ExecuteTemplate(rw, "bank-id.html", qrCode)
		}
	})

	mux.HandleFunc("/status", func(rw http.ResponseWriter, r *http.Request) {
		template, err := template.ParseGlob("templates/*")
		if err != nil {
			panic(err)
		}

		isFinished, qrCode, err := authenticator.State()
		if err != nil {
			template.ExecuteTemplate(rw, "error.html", err)
		} else if isFinished {
			template.ExecuteTemplate(rw, "finished.html", nil)
		} else {
			template.ExecuteTemplate(rw, "bank-id.html", qrCode)
		}
	})

	return mux
}
