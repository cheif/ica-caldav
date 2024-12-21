package main

import (
	"embed"
	"ica-caldav/ica"
	"io"
	"net/http"
	"text/template"
)

func newServerForSetup(authenticator *ica.BankIDAuthenticator) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		executeTemplate(rw, "index.html", getState(authenticator))
	})

	mux.HandleFunc("/start", func(rw http.ResponseWriter, r *http.Request) {
		err := authenticator.Start()
		if err != nil {
			state := SetupState{
				Error: err,
			}
			executeTemplate(rw, "status", state)
		} else {
			executeTemplate(rw, "status", getState(authenticator))
		}
	})

	mux.HandleFunc("/status", func(rw http.ResponseWriter, r *http.Request) {
		executeTemplate(rw, "status", getState(authenticator))
	})

	return mux
}

type SetupState struct {
	Started    bool
	IsComplete bool
	Error      error
	QRCode     string
}

func getState(authenticator *ica.BankIDAuthenticator) SetupState {
	if authenticator.HasValidSession() {
		return SetupState{
			Started:    true,
			IsComplete: true,
		}
	} else if !authenticator.HasStarted() {
		return SetupState{
			Started: false,
		}
	} else {
		isFinished, qrCode, err := authenticator.Poll()
		if isFinished {
			return SetupState{
				Started:    true,
				IsComplete: true,
			}
		} else if err != nil {
			return SetupState{
				Started: true,
				Error:   err,
			}
		} else {
			return SetupState{
				Started: true,
				QRCode:  qrCode,
			}
		}
	}
}

//go:embed templates
var templates embed.FS

// It's probably quicker to just fetch templates once, but this makes it support live-reload.
func executeTemplate(rw io.Writer, name string, val any) {
	template, err := template.ParseFS(templates, "templates/*")
	if err != nil {
		panic(err)
	}
	template.ExecuteTemplate(rw, name, val)
}
